package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"raccoon_lab/internal/repositorio"
)

var reGrupo = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.-]{0,14}$`)

func codigoGrupo(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// ListarGrupos: GET /api/grupos  (ADMIN: todos | PROFESOR: solo los suyos)
func (a *App) ListarGrupos(w http.ResponseWriter, r *http.Request) {
	s := sesionDe(r)
	docente := ""
	if s.RolActivo != "ADMIN" {
		docente = s.Matricula
	}
	grupos, err := a.Grupos.Listar(r.Context(), docente)
	if err != nil {
		log.Printf("[GRUPOS] %v", err)
		responderError(w, http.StatusInternalServerError, "No se pudieron cargar los grupos")
		return
	}
	responderJSON(w, http.StatusOK, grupos)
}

// AlumnosDeGrupo: GET /api/grupos/{codigo}/alumnos  (el maestro solo ve sus grupos)
func (a *App) AlumnosDeGrupo(w http.ResponseWriter, r *http.Request) {
	s := sesionDe(r)
	codigo := codigoGrupo(r.PathValue("codigo"))

	if s.RolActivo != "ADMIN" {
		ok, err := a.Grupos.EsDocente(r.Context(), codigo, s.Matricula)
		if err != nil {
			responderError(w, http.StatusInternalServerError, "Error al consultar la base de datos")
			return
		}
		if !ok {
			responderError(w, http.StatusForbidden, "No estás asignado a este grupo")
			return
		}
	}

	lista, err := a.Grupos.Alumnos(r.Context(), codigo)
	if err != nil {
		log.Printf("[GRUPOS] %v", err)
		responderError(w, http.StatusInternalServerError, "No se pudieron cargar los alumnos")
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

// MaestrosDisponibles: GET /api/admin/maestros
func (a *App) MaestrosDisponibles(w http.ResponseWriter, r *http.Request) {
	lista, err := a.Grupos.MaestrosDisponibles(r.Context())
	if err != nil {
		log.Printf("[GRUPOS] %v", err)
		responderError(w, http.StatusInternalServerError, "No se pudo cargar la lista de maestros")
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

// CrearGrupo: POST /api/admin/grupos  { codigo }
func (a *App) CrearGrupo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Codigo string `json:"codigo"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	codigo := codigoGrupo(req.Codigo)
	if !reGrupo.MatchString(codigo) {
		responderError(w, http.StatusBadRequest, "Código inválido: letras, números, guion, punto o guion bajo (máx. 15)")
		return
	}
	err := a.Grupos.Crear(r.Context(), codigo)
	switch {
	case errors.Is(err, repositorio.ErrGrupoExiste):
		responderError(w, http.StatusConflict, "Ya existe el grupo "+codigo)
		return
	case err != nil:
		log.Printf("[GRUPOS] crear %s: %v", codigo, err)
		responderError(w, http.StatusInternalServerError, "No se pudo crear el grupo")
		return
	}
	log.Printf("[GRUPOS] %s creó %s", sesionDe(r).Matricula, codigo)
	responderJSON(w, http.StatusCreated, map[string]string{"codigo": codigo})
}

// EliminarGrupo: DELETE /api/admin/grupos/{codigo}
func (a *App) EliminarGrupo(w http.ResponseWriter, r *http.Request) {
	codigo := codigoGrupo(r.PathValue("codigo"))
	alumnos, err := a.Grupos.Eliminar(r.Context(), codigo)
	switch {
	case errors.Is(err, repositorio.ErrGrupoConAlumno):
		responderError(w, http.StatusConflict,
			fmt.Sprintf("El grupo %s tiene %d alumno(s). Muévelos a otro grupo (reimportando) antes de eliminarlo", codigo, alumnos))
		return
	case errors.Is(err, repositorio.ErrGrupoNoExiste):
		responderError(w, http.StatusNotFound, "El grupo no existe")
		return
	case err != nil:
		log.Printf("[GRUPOS] eliminar %s: %v", codigo, err)
		responderError(w, http.StatusInternalServerError, "No se pudo eliminar el grupo")
		return
	}
	log.Printf("[GRUPOS] %s eliminó %s", sesionDe(r).Matricula, codigo)
	w.WriteHeader(http.StatusNoContent)
}

// AsignarDocentes: PUT /api/admin/grupos/{codigo}/docentes  { docentes: [...] }
func (a *App) AsignarDocentes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Docentes []string `json:"docentes"`
	}
	if err := leerJSON(r, &req); err != nil {
		responderError(w, http.StatusBadRequest, "Solicitud inválida")
		return
	}
	codigo := codigoGrupo(r.PathValue("codigo"))

	docentes := make([]string, 0, len(req.Docentes))
	for _, m := range req.Docentes {
		m = strings.ToLower(strings.TrimSpace(m))
		if m != "" && !slices.Contains(docentes, m) {
			docentes = append(docentes, m)
		}
	}

	invalidos, err := a.Grupos.AsignarDocentes(r.Context(), codigo, docentes)
	switch {
	case errors.Is(err, repositorio.ErrGrupoNoExiste):
		responderError(w, http.StatusNotFound, "El grupo no existe")
		return
	case err != nil:
		log.Printf("[GRUPOS] asignar %s: %v", codigo, err)
		responderError(w, http.StatusInternalServerError, "No se pudo guardar la asignación")
		return
	case len(invalidos) > 0:
		responderError(w, http.StatusBadRequest,
			"No son maestros activos: "+strings.Join(invalidos, ", "))
		return
	}
	log.Printf("[GRUPOS] %s asignó a %s: %v", sesionDe(r).Matricula, codigo, docentes)
	w.WriteHeader(http.StatusNoContent)
}
