package api

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"

	"raccoon_lab/internal/labs"
)

// hostDe: la IP o nombre con el que el usuario abrió la plataforma.
// Así el enlace a GNS3 sirve aunque la IP del servidor cambie.
func hostDe(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		return h
	}
	return r.Host
}

func (a *App) errorLabs(w http.ResponseWriter, err error, contexto string) {
	switch {
	case errors.Is(err, labs.ErrYaActivo):
		responderError(w, http.StatusConflict, err.Error())
	case errors.Is(err, labs.ErrSinLaboratorio), errors.Is(err, labs.ErrCodigoInvalido):
		responderError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, labs.ErrSinPuertos):
		responderError(w, http.StatusServiceUnavailable, err.Error())
	default:
		log.Printf("[LABS] %s: %v", contexto, err)
		responderError(w, http.StatusInternalServerError, "Error del laboratorio: "+err.Error())
	}
}

// EstadoLab: GET /api/labs/estado
func (a *App) EstadoLab(w http.ResponseWriter, r *http.Request) {
	e, err := a.Labs.Estado(r.Context(), sesionDe(r).Matricula, hostDe(r))
	if err != nil {
		a.errorLabs(w, err, "estado")
		return
	}
	responderJSON(w, http.StatusOK, e)
}

// CrearLab: POST /api/labs/crear  { es_colaborativo } o { codigo_lab }
func (a *App) CrearLab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EsColaborativo bool   `json:"es_colaborativo"`
		CodigoLab      string `json:"codigo_lab"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	s := sesionDe(r)

	if req.CodigoLab != "" {
		e, err := a.Labs.Unirse(r.Context(), s.Matricula, hostDe(r), req.CodigoLab)
		if err != nil {
			a.errorLabs(w, err, "unirse")
			return
		}
		responderJSON(w, http.StatusOK, e)
		return
	}

	// Aunque el usuario cierre la página, la creación termina (evita contenedores a medias)
	e, err := a.Labs.Crear(context.WithoutCancel(r.Context()), s.Matricula, hostDe(r), req.EsColaborativo)
	if err != nil {
		a.errorLabs(w, err, "crear")
		return
	}
	responderJSON(w, http.StatusCreated, e)
}

// LimpiarLab: POST /api/labs/limpiar
func (a *App) LimpiarLab(w http.ResponseWriter, r *http.Request) {
	msg, err := a.Labs.Limpiar(r.Context(), sesionDe(r).Matricula)
	if err != nil {
		a.errorLabs(w, err, "limpiar")
		return
	}
	responderJSON(w, http.StatusOK, map[string]string{"message": msg})
}
