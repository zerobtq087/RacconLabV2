package api

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"raccoon_lab/internal/repositorio"

	"golang.org/x/crypto/bcrypt"
)

// entero lee un número de la URL y lo limita entre lo y hi
func entero(r *http.Request, clave string, porDefecto, lo, hi int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(clave))
	if err != nil {
		return porDefecto
	}
	return max(lo, min(n, hi))
}

// ListarUsuarios: GET /api/admin/usuarios?pagina=&por_pagina=&q=&tipo=&rol=&grupo=&orden=&desc=
func (a *App) ListarUsuarios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := &repositorio.FiltroUsuarios{
		Pagina:    entero(r, "pagina", 1, 1, 1_000_000),
		PorPagina: entero(r, "por_pagina", 10, 1, 100),
		Busqueda:  q.Get("q"),
		Tipo:      q.Get("tipo"),
		Rol:       strings.ToUpper(q.Get("rol")),
		Grupo:     strings.ToUpper(q.Get("grupo")),
		Orden:     q.Get("orden"),
		Desc:      q.Get("desc") == "true",
	}
	pagina, err := a.Usuarios.Listar(r.Context(), f)
	if err != nil {
		log.Printf("[USUARIOS] %v", err)
		responderError(w, http.StatusInternalServerError, "No se pudieron cargar los usuarios")
		return
	}
	responderJSON(w, http.StatusOK, pagina)
}

// ResumenUsuarios: GET /api/admin/usuarios/resumen -> tarjetas + grupos
func (a *App) ResumenUsuarios(w http.ResponseWriter, r *http.Request) {
	res, err := a.Usuarios.Resumen(r.Context())
	if err != nil {
		log.Printf("[USUARIOS] %v", err)
		responderError(w, http.StatusInternalServerError, "No se pudo cargar el resumen")
		return
	}
	responderJSON(w, http.StatusOK, res)
}

// EditarUsuario: PUT /api/admin/usuarios/editar  { matricula, roles, activo }
func (a *App) EditarUsuario(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Matricula string   `json:"matricula"`
		Roles     []string `json:"roles"`
		Activo    *bool    `json:"activo"` // puntero: distingue "false" de "no vino"
	}
	if err := leerJSON(r, &req); err != nil || req.Activo == nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	matricula := strings.ToLower(strings.TrimSpace(req.Matricula))

	// Normalizar roles: solo válidos, sin repetir, ALUMNO siempre
	roles := []string{"ALUMNO"}
	for _, rol := range req.Roles {
		rol = strings.ToUpper(strings.TrimSpace(rol))
		if (rol == "ADMIN" || rol == "PROFESOR") && !slices.Contains(roles, rol) {
			roles = append(roles, rol)
		}
	}

	yo := sesionDe(r)
	esYo := matricula == yo.Matricula
	if esYo && (!slices.Contains(roles, "ADMIN") || !*req.Activo) {
		responderError(w, http.StatusBadRequest, "No puedes quitarte el rol de Admin ni desactivar tu propia cuenta")
		return
	}

	err := a.Usuarios.ActualizarRoles(r.Context(), matricula, roles, *req.Activo)
	switch {
	case errors.Is(err, repositorio.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	case errors.Is(err, repositorio.ErrUltimoAdmin):
		responderError(w, http.StatusBadRequest, "Debe quedar al menos un admin activo")
		return
	case err != nil:
		log.Printf("[USUARIOS] editar %s: %v", matricula, err)
		responderError(w, http.StatusInternalServerError, "No se pudieron guardar los cambios")
		return
	}

	// Sus sesiones tienen los roles viejos: se cierran (menos la del admin que edita)
	if !esYo {
		_ = a.Sesiones.EliminarTodas(r.Context(), matricula)
	}
	log.Printf("[USUARIOS] %s cambió %s -> roles=%v activo=%v", yo.Matricula, matricula, roles, *req.Activo)

	u, _ := a.Usuarios.BuscarPorMatricula(r.Context(), matricula)
	responderJSON(w, http.StatusOK, u)
}

// ResetPassword: PUT /api/admin/usuarios/password  { matricula, nueva_password }
func (a *App) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Matricula string `json:"matricula"`
		Nueva     string `json:"nueva_password"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	matricula := strings.ToLower(strings.TrimSpace(req.Matricula))
	switch {
	case len(req.Nueva) < 8:
		responderError(w, http.StatusBadRequest, "La contraseña debe tener al menos 8 caracteres")
		return
	case len(req.Nueva) > 72:
		responderError(w, http.StatusBadRequest, "La contraseña no puede pasar de 72 caracteres")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Nueva), bcrypt.DefaultCost)
	if err != nil {
		responderError(w, http.StatusInternalServerError, "No se pudo cifrar la contraseña")
		return
	}
	err = a.Usuarios.CambiarPassword(r.Context(), matricula, string(hash))
	switch {
	case errors.Is(err, repositorio.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	case err != nil:
		log.Printf("[USUARIOS] password %s: %v", matricula, err)
		responderError(w, http.StatusInternalServerError, "No se pudo cambiar la contraseña")
		return
	}

	// La contraseña anterior deja de servir: se cierran todas sus sesiones
	_ = a.Sesiones.EliminarTodas(r.Context(), matricula)
	a.Limitador.Exito(matricula) // si estaba bloqueado por intentos, se desbloquea
	log.Printf("[USUARIOS] %s restableció la contraseña de %s", sesionDe(r).Matricula, matricula)
	w.WriteHeader(http.StatusNoContent)
}
