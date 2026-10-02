package api

import (
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// ObtenerPerfil: GET /api/usuario/perfil
func (a *App) ObtenerPerfil(w http.ResponseWriter, r *http.Request) {
	s := sesionDe(r)
	u, err := a.Usuarios.BuscarPorMatricula(r.Context(), s.Matricula)
	if err != nil || u == nil {
		responderError(w, http.StatusNotFound, "No se encontró tu usuario")
		return
	}

	// Grupos donde da clase (solo si tiene rol Maestro)
	gruposDocente := []string{}
	if u.TieneRol("PROFESOR") {
		grupos, err := a.Grupos.Listar(r.Context(), u.Matricula)
		if err != nil {
			log.Printf("[PERFIL] grupos de %s: %v", u.Matricula, err)
		}
		for _, g := range grupos {
			gruposDocente = append(gruposDocente, g.Codigo)
		}
	}

	responderJSON(w, http.StatusOK, map[string]any{
		"matricula":      u.Matricula,
		"nombre":         u.Nombre,
		"tipo":           u.Tipo,
		"grupo":          u.Grupo,
		"roles":          u.Roles,
		"grupos_docente": gruposDocente,
	})
}

// CambiarMiPassword: PUT /api/usuario/perfil/password  { password_actual, nueva_password }
func (a *App) CambiarMiPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actual string `json:"password_actual"`
		Nueva  string `json:"nueva_password"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	s := sesionDe(r)

	switch {
	case len(req.Nueva) < 8:
		responderError(w, http.StatusBadRequest, "La nueva contraseña debe tener al menos 8 caracteres")
		return
	case len(req.Nueva) > 72:
		responderError(w, http.StatusBadRequest, "La nueva contraseña no puede pasar de 72 caracteres")
		return
	case req.Nueva == req.Actual:
		responderError(w, http.StatusBadRequest, "La nueva contraseña debe ser distinta a la actual")
		return
	}

	// Mismo bloqueo que el login: 5 contraseñas actuales incorrectas = 15 min
	if bloqueado, resta := a.Limitador.Bloqueado(s.Matricula); bloqueado {
		responderBloqueo(w, resta)
		return
	}

	u, err := a.Usuarios.BuscarPorMatricula(r.Context(), s.Matricula)
	if err != nil || u == nil {
		responderError(w, http.StatusNotFound, "No se encontró tu usuario")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Actual)) != nil {
		if _, bloqueo := a.Limitador.Fallo(s.Matricula); bloqueo > 0 {
			responderBloqueo(w, bloqueo)
			return
		}
		responderError(w, http.StatusBadRequest, "La contraseña actual no es correcta")
		return
	}
	a.Limitador.Exito(s.Matricula)

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Nueva), bcrypt.DefaultCost)
	if err != nil {
		responderError(w, http.StatusInternalServerError, "No se pudo cifrar la contraseña")
		return
	}
	if err := a.Usuarios.CambiarPassword(r.Context(), s.Matricula, string(hash)); err != nil {
		log.Printf("[PERFIL] password %s: %v", s.Matricula, err)
		responderError(w, http.StatusInternalServerError, "No se pudo cambiar la contraseña")
		return
	}

	// Se cierran sus otras sesiones (otros navegadores); esta sigue abierta
	_ = a.Sesiones.EliminarOtras(r.Context(), s)
	log.Printf("[PERFIL] %s cambió su contraseña", s.Matricula)
	w.WriteHeader(http.StatusNoContent)
}
