package handler

import (
	"errors"
	"log"
	"strings"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/middleware"
	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

type UsuarioHandler struct {
	session    *middleware.Session
	repository *repository.UsuarioRepository
	usuarios   *service.Usuarios
	adminUser  string // matrícula del admin general (protegido)
}

func NewUsuarioHandler(s *middleware.Session, r *repository.UsuarioRepository, u *service.Usuarios, adminUser string) *UsuarioHandler {
	return &UsuarioHandler{session: s, repository: r, usuarios: u, adminUser: strings.ToLower(adminUser)}
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

// Tipos: tipos de usuario con los roles que da cada uno (para el formulario)
func (h *UsuarioHandler) Tipos(c fiber.Ctx) error {
	return c.JSON(h.repository.Tipos())
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

// errorUsuario traduce los errores del servicio a su status
func errorUsuario(c fiber.Ctx, err error) error {
	var ev *service.ErrValidacion
	switch {
	case errors.As(err, &ev):
		return c.Status(fiber.StatusBadRequest).SendString(ev.Texto)
	case errors.Is(err, repository.ErrMatriculaOcupada):
		return c.Status(fiber.StatusConflict).SendString(err.Error())
	}
	log.Printf("[USUARIOS] %v", err)
	return c.Status(fiber.StatusInternalServerError).SendString("No se pudo guardar")
}

func (h *UsuarioHandler) Create(c fiber.Ctx) error {
	var f domain.UsuarioForm
	if err := c.Bind().Body(&f); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	if err := h.usuarios.Create(&f); err != nil {
		return errorUsuario(c, err)
	}
	return c.SendString("success")
}

func (h *UsuarioHandler) Update(c fiber.Ctx) error {
	actual, err := h.usuarioEditable(c)
	if actual == nil {
		return err
	}

	var f domain.UsuarioForm
	if err := c.Bind().Body(&f); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	if userID, _ := c.Locals("userID").(string); userID == actual.ID && !f.Activo {
		return c.Status(fiber.StatusBadRequest).SendString("No puedes desactivar tu propia cuenta")
	}
	if err := h.usuarios.Update(actual, &f); err != nil {
		return errorUsuario(c, err)
	}
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
