package handler

import (
	"errors"
	"slices"
	"strings"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/middleware"
	"raccoon-lab/backend/repository"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

var rolesValidos = []string{"ADMIN", "PROFESOR", "ALUMNO"}

type UsuarioHandler struct {
	session    *middleware.Session
	repository *repository.UsuarioRepository
	adminUser  string // matrícula del admin general (protegido)
}

func NewUsuarioHandler(s *middleware.Session, r *repository.UsuarioRepository, adminUser string) *UsuarioHandler {
	return &UsuarioHandler{session: s, repository: r, adminUser: strings.ToLower(adminUser)}
}

func (h *UsuarioHandler) GetPage(c fiber.Ctx) error {
	f := domain.FiltroUsuarios{Pagina: 1, PorPagina: 10}
	if err := c.Bind().Query(&f); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Filtros inválidos")
	}
	if f.Pagina < 1 {
		f.Pagina = 1
	}
	if f.PorPagina < 1 || f.PorPagina > 100 {
		f.PorPagina = 100 // la tabla manda -1 cuando eligen "Todos"
	}

	pagina := h.repository.GetPage(&f)
	for i := range pagina.Usuarios {
		pagina.Usuarios[i].Protegido = pagina.Usuarios[i].Matricula == h.adminUser
	}
	return c.JSON(pagina)
}

// usuarioEditable regresa el usuario si existe y no es el admin general
func (h *UsuarioHandler) usuarioEditable(c fiber.Ctx) (*domain.Usuario, error) {
	u := h.repository.GetByID(c.Params("id"))
	if u == nil {
		return nil, c.Status(fiber.StatusNotFound).SendString("El usuario no existe")
	}
	if u.Matricula == h.adminUser {
		return nil, c.Status(fiber.StatusForbidden).SendString("El administrador general no se puede modificar")
	}
	return u, nil
}

func (h *UsuarioHandler) Update(c fiber.Ctx) error {
	actual, err := h.usuarioEditable(c)
	if actual == nil {
		return err
	}

	var u domain.Usuario
	if err := c.Bind().Body(&u); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	u.ID = actual.ID

	if strings.TrimSpace(u.Matricula) == "" || strings.TrimSpace(u.Nombre) == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Matrícula y nombre son obligatorios")
	}
	if len(u.Roles) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("El usuario debe tener al menos un rol")
	}
	for _, rol := range u.Roles {
		if !slices.Contains(rolesValidos, rol) {
			return c.Status(fiber.StatusBadRequest).SendString("Rol inválido: " + rol)
		}
	}
	if actual.Tipo != "alumnos" {
		u.Grupo = "" // solo los alumnos tienen grupo
	}

	if err := h.repository.Update(&u); err != nil {
		if errors.Is(err, repository.ErrMatriculaOcupada) {
			return c.Status(fiber.StatusConflict).SendString(err.Error())
		}
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo guardar")
	}

	// Sus permisos cambiaron: que vuelva a iniciar sesión
	_ = h.session.CloseAll(c.Context(), u.ID, "")
	return c.SendString("success")
}

func (h *UsuarioHandler) UpdatePasswd(c fiber.Ctx) error {
	u, err := h.usuarioEditable(c)
	if u == nil {
		return err
	}

	var req map[string]string
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	if len(req["password"]) < 8 {
		return c.Status(fiber.StatusBadRequest).SendString("La contraseña debe tener al menos 8 caracteres")
	}

	hashPasswd, _ := bcrypt.GenerateFromPassword([]byte(req["password"]), bcrypt.DefaultCost)
	if err := h.repository.UpdatePasswd(u.ID, string(hashPasswd)); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo cambiar la contraseña")
	}

	_ = h.session.CloseAll(c.Context(), u.ID, "")
	return c.SendString("success")
}

func (h *UsuarioHandler) Delete(c fiber.Ctx) error {
	u, err := h.usuarioEditable(c)
	if u == nil {
		return err
	}

	if userID, _ := c.Locals("userID").(string); userID == u.ID {
		return c.Status(fiber.StatusBadRequest).SendString("No puedes borrar tu propia cuenta")
	}

	if err := h.repository.Delete(u.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo borrar")
	}

	_ = h.session.CloseAll(c.Context(), u.ID, "")
	return c.SendString("success")
}

func (h *UsuarioHandler) Maestros(c fiber.Ctx) error {
	return c.JSON(h.repository.Maestros())
}
