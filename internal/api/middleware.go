package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"raccoon_lab/internal/modelos"
)

const cookieSesion = "rl_sesion"

type claveCtx struct{}

// sesionDe obtiene la sesión que el middleware guardó en el contexto
func sesionDe(r *http.Request) *modelos.Sesion {
	s, _ := r.Context().Value(claveCtx{}).(*modelos.Sesion)
	return s
}

// tokenDe lee el id de sesión del header Authorization o de la cookie
func tokenDe(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if c, err := r.Cookie(cookieSesion); err == nil {
		return c.Value
	}
	return ""
}

// ConSesion exige una sesión válida en Redis
func (a *App) ConSesion(siguiente http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := a.Sesiones.Obtener(r.Context(), tokenDe(r))
		if err != nil {
			responderError(w, http.StatusUnauthorized, "Sesión no válida o expirada")
			return
		}
		ctx := context.WithValue(r.Context(), claveCtx{}, s)
		siguiente(w, r.WithContext(ctx))
	})
}

// ConRol exige sesión Y que el rol ACTIVO sea uno de los permitidos
func (a *App) ConRol(siguiente http.HandlerFunc, roles ...string) http.Handler {
	return a.ConSesion(func(w http.ResponseWriter, r *http.Request) {
		if s := sesionDe(r); !slices.Contains(roles, s.RolActivo) {
			responderError(w, http.StatusForbidden, "No tienes permiso para esta acción con tu rol actual")
			return
		}
		siguiente(w, r)
	})
}
