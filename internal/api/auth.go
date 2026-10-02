package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"raccoon_lab/internal/modelos"

	"golang.org/x/crypto/bcrypt"
)

type solicitudLogin struct {
	Matricula string `json:"matricula"`
	Password  string `json:"password"`
	Rol       string `json:"rol"`
}

// respuestaSesion es lo que espera el frontend: { access_token, usuario }
func respuestaSesion(s *modelos.Sesion) map[string]any {
	return map[string]any{
		"access_token": s.ID,
		"usuario":      s,
	}
}

func (a *App) ponerCookie(w http.ResponseWriter, id string, duracion time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSesion,
		Value:    id,
		Path:     "/",
		MaxAge:   int(duracion.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// responderBloqueo manda los segundos exactos para que el frontend
// muestre la cuenta regresiva en tiempo real
func responderBloqueo(w http.ResponseWriter, resta time.Duration) {
	seg := int(resta.Seconds())
	w.Header().Set("Retry-After", strconv.Itoa(seg))
	responderJSON(w, http.StatusTooManyRequests, map[string]any{
		"message":   "Demasiados intentos fallidos. Cuenta bloqueada temporalmente",
		"bloqueo_s": seg,
	})
}

// Login: valida en FIREBIRD, crea la sesión en REDIS
func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	var req solicitudLogin
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	matricula := strings.ToLower(strings.TrimSpace(req.Matricula))
	rol := strings.ToUpper(strings.TrimSpace(req.Rol))
	if matricula == "" || req.Password == "" || rol == "" {
		responderError(w, http.StatusBadRequest, "Matrícula, contraseña y rol son obligatorios")
		return
	}

	if bloqueado, resta := a.Limitador.Bloqueado(matricula); bloqueado {
		responderBloqueo(w, resta)
		return
	}

	u, err := a.Usuarios.BuscarPorMatricula(r.Context(), matricula)
	if err != nil {
		log.Printf("[LOGIN] error de BD: %v", err)
		responderError(w, http.StatusInternalServerError, "Error al consultar la base de datos")
		return
	}

	// Mismo mensaje si no existe o si la contraseña es incorrecta (no revelar cuál)
	if u == nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		restantes, bloqueo := a.Limitador.Fallo(matricula)
		if bloqueo > 0 {
			responderBloqueo(w, bloqueo)
			return
		}
		responderJSON(w, http.StatusUnauthorized, map[string]any{
			"message":            fmt.Sprintf("Matrícula/clave o contraseña incorrectas. Te quedan %d intentos", restantes),
			"intentos_restantes": restantes,
		})
		return
	}
	if !u.Activo {
		responderError(w, http.StatusForbidden, "Tu cuenta está desactivada. Contacta al administrador")
		return
	}
	if !u.TieneRol(rol) {
		responderError(w, http.StatusForbidden, "No tienes asignado ese rol")
		return
	}

	a.Limitador.Exito(matricula)

	s := &modelos.Sesion{
		Matricula: u.Matricula,
		Nombre:    u.Nombre,
		Tipo:      u.Tipo,
		Grupo:     u.Grupo,
		Roles:     u.Roles,
		RolActivo: rol,
	}
	if err := a.Sesiones.Crear(r.Context(), s); err != nil {
		log.Printf("[LOGIN] error de Redis: %v", err)
		responderError(w, http.StatusInternalServerError, "No se pudo crear la sesión")
		return
	}

	go func(m string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.Usuarios.RegistrarAcceso(ctx, m)
	}(u.Matricula)

	log.Printf("[LOGIN] %s entró como %s", u.Matricula, rol)
	a.ponerCookie(w, s.ID, a.Cfg.DuracionSesion)
	responderJSON(w, http.StatusOK, respuestaSesion(s))
}

// Refresh: al recargar la página, recupera la sesión desde la cookie
func (a *App) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieSesion)
	if err != nil {
		responderError(w, http.StatusUnauthorized, "Sin sesión")
		return
	}
	s, err := a.Sesiones.Obtener(r.Context(), c.Value)
	if err != nil {
		a.ponerCookie(w, "", -time.Second)
		responderError(w, http.StatusUnauthorized, "Sesión expirada")
		return
	}
	a.ponerCookie(w, s.ID, a.Cfg.DuracionSesion)
	responderJSON(w, http.StatusOK, respuestaSesion(s))
}

// Logout: borra la sesión de Redis y la cookie
func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	if s, err := a.Sesiones.Obtener(r.Context(), tokenDe(r)); err == nil {
		_ = a.Sesiones.Eliminar(r.Context(), s)
	}
	a.ponerCookie(w, "", -time.Second)
	w.WriteHeader(http.StatusNoContent)
}

// CambiarRol: cambia el rol activo sin volver a iniciar sesión
func (a *App) CambiarRol(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rol string `json:"rol"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	s := sesionDe(r)
	rol := strings.ToUpper(strings.TrimSpace(req.Rol))
	if !s.TieneRol(rol) {
		responderError(w, http.StatusForbidden, "No tienes asignado ese rol")
		return
	}
	if err := a.Sesiones.CambiarRol(r.Context(), s, rol); err != nil {
		responderError(w, http.StatusInternalServerError, "No se pudo cambiar el rol")
		return
	}
	responderJSON(w, http.StatusOK, respuestaSesion(s))
}
