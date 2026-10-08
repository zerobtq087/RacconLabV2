package api

import (
	"context"
	"errors"
	"net/http"

	"raccoon_lab/internal/labs"
)

// DesplegarPlantilla: POST /api/labs/profesor/despliegue
// Corre en el laboratorio del maestro (el que tiene en "Mi laboratorio").
func (a *App) DesplegarPlantilla(w http.ResponseWriter, r *http.Request) {
	var req struct {
		labs.Despliegue
		// El frontend también manda estos; se ignoran: el lab sale de la sesión
		ContainerName string `json:"container_name"`
		PuertoGNS3    int    `json:"puerto_gns3"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}

	// Aunque el maestro cierre la página, Ansible termina (no deja la topología a medias)
	msg, err := a.Labs.DesplegarPlantilla(context.WithoutCancel(r.Context()), sesionDe(r).Matricula, &req.Despliegue)
	switch {
	case errors.Is(err, labs.ErrSinLaboratorio):
		responderError(w, http.StatusNotFound, "Primero despliega tu laboratorio en “Mi laboratorio”")
	case errors.Is(err, labs.ErrNoEsDueno):
		responderError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, labs.ErrDespliegueEnCurso):
		responderError(w, http.StatusConflict, err.Error())
	case err != nil:
		responderError(w, http.StatusBadRequest, err.Error())
	default:
		responderJSON(w, http.StatusOK, map[string]string{"message": msg})
	}
}
