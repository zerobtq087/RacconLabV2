package api

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"raccoon_lab/internal/repositorio"
)

// Mismas reglas que la importación
var reMatricula = regexp.MustCompile(`^[a-z0-9._-]{1,20}$`)

// EditarDatos: PUT /api/admin/usuarios/datos
// { matricula (actual), nueva_matricula, nombre, grupo }
func (a *App) EditarDatos(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Matricula      string  `json:"matricula"`
		NuevaMatricula string  `json:"nueva_matricula"`
		Nombre         string  `json:"nombre"`
		Grupo          *string `json:"grupo"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}

	actual := strings.ToLower(strings.TrimSpace(req.Matricula))
	d := &repositorio.DatosUsuario{
		Matricula: strings.ToLower(strings.TrimSpace(req.NuevaMatricula)),
		Nombre:    strings.Join(strings.Fields(req.Nombre), " "),
	}
	if d.Matricula == "" {
		d.Matricula = actual
	}
	if req.Grupo != nil {
		if g := strings.ToUpper(strings.TrimSpace(*req.Grupo)); g != "" {
			d.Grupo = &g
		}
	}

	yo := sesionDe(r)
	switch {
	case actual == a.Cfg.AdminClave:
		responderError(w, http.StatusForbidden, "El administrador general no se puede modificar")
		return
	case actual == yo.Matricula && d.Matricula != actual:
		responderError(w, http.StatusBadRequest, "No puedes cambiar tu propia matrícula/clave")
		return
	case !reMatricula.MatchString(d.Matricula):
		responderError(w, http.StatusBadRequest, "Matrícula/clave inválida: solo letras, números, . _ - (máx. 20)")
		return
	case d.Nombre == "":
		responderError(w, http.StatusBadRequest, "El nombre es obligatorio")
		return
	case utf8.RuneCountInString(d.Nombre) > 120:
		responderError(w, http.StatusBadRequest, "El nombre pasa de 120 caracteres")
		return
	case d.Grupo != nil && !reGrupo.MatchString(*d.Grupo):
		responderError(w, http.StatusBadRequest, "Grupo inválido: letras, números, guion, punto o guion bajo (máx. 15)")
		return
	}

	err := a.Usuarios.ActualizarDatos(r.Context(), actual, d)
	switch {
	case errors.Is(err, repositorio.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	case errors.Is(err, repositorio.ErrProtegido):
		responderError(w, http.StatusForbidden, "El administrador general no se puede modificar")
		return
	case errors.Is(err, repositorio.ErrMatriculaOcupada):
		responderError(w, http.StatusConflict, "Ya existe un usuario con la matrícula/clave "+d.Matricula)
		return
	case err != nil:
		log.Printf("[USUARIOS] datos %s: %v", actual, err)
		responderError(w, http.StatusInternalServerError, "No se pudieron guardar los datos")
		return
	}

	// Su sesión guarda matrícula, nombre y grupo viejos: se cierra (menos la propia)
	if actual != yo.Matricula {
		_ = a.Sesiones.EliminarTodas(r.Context(), actual)
	}
	log.Printf("[USUARIOS] %s editó %s -> %s, %q, grupo=%v", yo.Matricula, actual, d.Matricula, d.Nombre, d.Grupo)

	u, _ := a.Usuarios.BuscarPorMatricula(r.Context(), d.Matricula)
	responderJSON(w, http.StatusOK, u)
}

// EliminarUsuario: DELETE /api/admin/usuarios/{matricula}
func (a *App) EliminarUsuario(w http.ResponseWriter, r *http.Request) {
	matricula := strings.ToLower(strings.TrimSpace(r.PathValue("matricula")))
	yo := sesionDe(r)

	if matricula == yo.Matricula {
		responderError(w, http.StatusBadRequest, "No puedes eliminar tu propia cuenta")
		return
	}

	err := a.Usuarios.Eliminar(r.Context(), matricula)
	switch {
	case errors.Is(err, repositorio.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	case errors.Is(err, repositorio.ErrProtegido):
		responderError(w, http.StatusForbidden, "El administrador general no se puede eliminar")
		return
	case errors.Is(err, repositorio.ErrUltimoAdmin):
		responderError(w, http.StatusBadRequest, "Debe quedar al menos un admin activo")
		return
	case err != nil:
		log.Printf("[USUARIOS] eliminar %s: %v", matricula, err)
		responderError(w, http.StatusInternalServerError, "No se pudo eliminar el usuario")
		return
	}

	_ = a.Sesiones.EliminarTodas(r.Context(), matricula)
	log.Printf("[USUARIOS] %s eliminó a %s", yo.Matricula, matricula)
	w.WriteHeader(http.StatusNoContent)
}
