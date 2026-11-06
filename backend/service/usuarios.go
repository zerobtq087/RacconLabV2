package service

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"

	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------
// Reglas de los datos de un usuario: las usan el importador (por fila)
// y el alta/edición manual, así las dos validan exactamente igual.
// ---------------------------------------------------------------------

var reMatricula = regexp.MustCompile(`^[a-z0-9._-]{1,20}$`)

var rolesValidos = []string{"ADMIN", "PROFESOR", "ALUMNO"}

// limpiarNombre quita espacios de más: "  Ana   López " -> "Ana López"
func limpiarNombre(s string) string { return strings.Join(strings.Fields(s), " ") }

// validarDatos revisa matrícula, nombre, grupo y carrera (la contraseña se revisa aparte)
// y los deja normalizados. Regresa "" si todo está bien o el mensaje del error.
func validarDatos(matricula, nombre, grupo, carrera *string, esAlumno bool) string {
	*matricula = strings.ToLower(strings.TrimSpace(*matricula))
	*nombre = limpiarNombre(*nombre)
	*grupo = strings.ToUpper(strings.TrimSpace(*grupo))
	*carrera = limpiarNombre(*carrera)

	switch {
	case *matricula == "":
		return "falta la matrícula"
	case !reMatricula.MatchString(*matricula):
		return fmt.Sprintf("matrícula inválida %q (solo letras, números, . _ -; máx. 20)", *matricula)
	case *nombre == "":
		return "falta el nombre"
	case utf8.RuneCountInString(*nombre) > 120:
		return "el nombre pasa de 120 caracteres"
	}
	if !esAlumno {
		*grupo, *carrera = "", "" // solo los alumnos tienen grupo
		return ""
	}
	switch {
	case *grupo == "":
		return "falta el grupo (es obligatorio para alumnos)"
	case utf8.RuneCountInString(*grupo) > 15:
		return "el grupo pasa de 15 caracteres"
	case utf8.RuneCountInString(*carrera) > 120:
		return "la carrera pasa de 120 caracteres"
	}
	return ""
}

// tipoDeTexto convierte lo que viene en la columna "tipo" (ALUMNO, PROFESOR, ADMIN
// y variantes) en el tipo de usuario de la base (alumnos, maestros, admins)
func tipoDeTexto(s string) (string, bool) {
	switch normalizar(s) {
	case "alumno", "alumnos", "estudiante", "estudiantes":
		return "alumnos", true
	case "profesor", "profesores", "maestro", "maestros", "docente", "docentes":
		return "maestros", true
	case "admin", "admins", "administrador", "administradores":
		return "admins", true
	}
	return "", false
}

// ---------------------------------------------------------------------
// Usuarios: alta y edición manual (Usuarios.vue). Solo ADMIN.
// ---------------------------------------------------------------------
type Usuarios struct {
	repository *repository.UsuarioRepository
	cerrar     CerrarSesiones
	protegida  string // matrícula del admin general: no se crea ni se edita aquí
}

func NewUsuarios(r *repository.UsuarioRepository, cerrar CerrarSesiones, protegida string) *Usuarios {
	return &Usuarios{repository: r, cerrar: cerrar, protegida: strings.ToLower(protegida)}
}

// validarForm revisa el formulario y completa los roles del tipo si no vienen
func (s *Usuarios) validarForm(f *domain.UsuarioForm, passwordObligatoria bool) error {
	tipos := s.repository.Tipos()
	i := slices.IndexFunc(tipos, func(t domain.TipoUsuario) bool { return t.Clave == f.Tipo })
	if i < 0 {
		return invalido("Elige el tipo de usuario (alumno, maestro o administrador)")
	}
	if msg := validarDatos(&f.Matricula, &f.Nombre, &f.Grupo, &f.Carrera, f.Tipo == "alumnos"); msg != "" {
		return invalido("%s", strings.ToUpper(msg[:1])+msg[1:])
	}
	if f.Matricula == s.protegida {
		return invalido("Esa matrícula es del administrador general")
	}
	if f.Password != "" || passwordObligatoria {
		if len(f.Password) < 8 || len(f.Password) > 72 {
			return invalido("La contraseña debe tener de 8 a 72 caracteres")
		}
	}

	if len(f.Roles) == 0 {
		f.Roles = tipos[i].Roles // los del catálogo TIPO_ROLES
	}
	for _, rol := range f.Roles {
		if !slices.Contains(rolesValidos, rol) {
			return invalido("Rol inválido: %s", rol)
		}
	}
	return nil
}

// Create da de alta un usuario a mano
func (s *Usuarios) Create(f *domain.UsuarioForm) error {
	if err := s.validarForm(f, true); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(f.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repository.Create(f, string(hash))
}

// Update edita un usuario. Si cambia la contraseña, el tipo, los roles o la
// matrícula, o se desactiva, se cierran sus sesiones (que vuelva a entrar).
func (s *Usuarios) Update(actual *domain.Usuario, f *domain.UsuarioForm) error {
	if err := s.validarForm(f, false); err != nil {
		return err
	}
	hash := ""
	if f.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(f.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hash = string(h)
	}
	if err := s.repository.Update(actual.ID, f, hash); err != nil {
		return err
	}

	rolesAntes := slices.Clone(actual.Roles)
	rolesAhora := slices.Clone(f.Roles)
	slices.Sort(rolesAntes)
	slices.Sort(rolesAhora)

	if hash != "" || f.Tipo != actual.Tipo || (actual.Activo && !f.Activo) ||
		!slices.Equal(rolesAntes, rolesAhora) || f.Matricula != actual.Matricula {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.cerrar(ctx, actual.ID)
	}
	return nil
}
