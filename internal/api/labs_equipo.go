package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"raccoon_lab/internal/labs"
)

// CompanerosLab: GET /api/labs/companeros
func (a *App) CompanerosLab(w http.ResponseWriter, r *http.Request) {
	s := sesionDe(r)
	lista, err := a.Labs.Companeros(r.Context(), s.Matricula, s.Grupo)
	if err != nil {
		a.errorLabs(w, err, "compañeros")
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

// InvitarLab: POST /api/labs/invitaciones/invitar  { invitados_ids: [...] }
func (a *App) InvitarLab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Invitados []string `json:"invitados_ids"`
	}
	if err := leerJSON(r, &req); err != nil || len(req.Invitados) == 0 {
		responderError(w, http.StatusBadRequest, "Indica a quién invitar")
		return
	}
	for i, m := range req.Invitados {
		req.Invitados[i] = strings.ToLower(strings.TrimSpace(m))
	}

	s := sesionDe(r)
	n, err := a.Labs.Invitar(r.Context(), s.Matricula, s.Grupo, req.Invitados)
	switch {
	case errors.Is(err, labs.ErrNoPuedeInvitar):
		responderError(w, http.StatusForbidden, err.Error())
		return
	case err != nil:
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("%d invitación(es) enviada(s)", n)})
}

// PendientesLab: GET /api/labs/invitaciones/pendientes
func (a *App) PendientesLab(w http.ResponseWriter, r *http.Request) {
	lista, err := a.Labs.Pendientes(r.Context(), sesionDe(r).Matricula)
	if err != nil {
		a.errorLabs(w, err, "pendientes")
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

// ResponderLab: POST /api/labs/invitaciones/responder  { id_invitacion, aceptar }
func (a *App) ResponderLab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string `json:"id_invitacion"`
		Aceptar bool   `json:"aceptar"`
	}
	if err := leerJSON(r, &req); err != nil || req.ID == "" {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	e, err := a.Labs.Responder(r.Context(), sesionDe(r).Matricula, hostDe(r), req.ID, req.Aceptar)
	switch {
	case errors.Is(err, labs.ErrInvitacionNoVale):
		responderError(w, http.StatusNotFound, err.Error())
		return
	case err != nil:
		a.errorLabs(w, err, "responder")
		return
	}
	if !req.Aceptar {
		responderJSON(w, http.StatusOK, map[string]string{"message": "Invitación rechazada"})
		return
	}
	responderJSON(w, http.StatusOK, e)
}
