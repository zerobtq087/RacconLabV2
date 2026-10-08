package api

import "net/http"

// ListarWorkspaces: GET /api/admin/workspaces
func (a *App) ListarWorkspaces(w http.ResponseWriter, r *http.Request) {
	lista, err := a.Labs.Todos(r.Context())
	if err != nil {
		a.errorLabs(w, err, "monitor")
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

// CerrarWorkspace: DELETE /api/admin/workspaces/{id}
func (a *App) CerrarWorkspace(w http.ResponseWriter, r *http.Request) {
	if err := a.Labs.Cerrar(r.Context(), r.PathValue("id"), sesionDe(r).Matricula); err != nil {
		a.errorLabs(w, err, "cerrar")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
