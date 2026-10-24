package handler

import (
	"errors"
	"log"

	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

type LaboratorioHandler struct {
	repository   *repository.LaboratorioRepository
	laboratorios *service.Laboratorios
}

func NewLaboratorioHandler(r *repository.LaboratorioRepository, s *service.Laboratorios) *LaboratorioHandler {
	return &LaboratorioHandler{repository: r, laboratorios: s}
}

// errorLab traduce los errores conocidos a 409 y el resto a 500
func errorLab(c fiber.Ctx, err error) error {
	conocidos := []error{
		repository.ErrYaEnLaboratorio, repository.ErrSinPuertos, repository.ErrCodigoInvalido,
		repository.ErrSinLaboratorio, repository.ErrNoEsDueno, repository.ErrNoEsEquipo, repository.ErrInvitacion,
		repository.ErrYaInvitado, repository.ErrCompaneroOcupado,
	}
	for _, e := range conocidos {
		if errors.Is(err, e) {
			return c.Status(fiber.StatusConflict).SendString(err.Error())
		}
	}
	log.Printf("[LABS] %v", err)
	return c.Status(fiber.StatusInternalServerError).SendString("No se pudo completar: " + err.Error())
}

func usuario(c fiber.Ctx) string {
	id, _ := c.Locals("userID").(string)
	return id
}

func (h *LaboratorioHandler) Status(c fiber.Ctx) error {
	return c.JSON(h.laboratorios.Status(usuario(c)))
}

func (h *LaboratorioHandler) Create(c fiber.Ctx) error {
	var req struct {
		Colaborativo bool `json:"colaborativo"`
	}
	_ = c.Bind().Body(&req)

	if err := h.laboratorios.Create(usuario(c), req.Colaborativo); err != nil {
		return errorLab(c, err)
	}
	return c.SendString("success")
}

func (h *LaboratorioHandler) Leave(c fiber.Ctx) error {
	if err := h.laboratorios.Leave(c.Context(), usuario(c)); err != nil {
		return errorLab(c, err)
	}
	return c.SendString("success")
}

func (h *LaboratorioHandler) Join(c fiber.Ctx) error {
	var req struct {
		Codigo string `json:"codigo"`
	}
	if err := c.Bind().Body(&req); err != nil || req.Codigo == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Escribe el código del equipo")
	}
	if err := h.repository.Join(usuario(c), req.Codigo); err != nil {
		return errorLab(c, err)
	}
	return c.SendString("success")
}

func (h *LaboratorioHandler) Companeros(c fiber.Ctx) error {
	return c.JSON(h.repository.Companeros(usuario(c)))
}

func (h *LaboratorioHandler) Invitations(c fiber.Ctx) error {
	return c.JSON(h.repository.Invitations(usuario(c)))
}

func (h *LaboratorioHandler) Invite(c fiber.Ctx) error {
	var req struct {
		IDUsuario string `json:"id_usuario"`
	}
	if err := c.Bind().Body(&req); err != nil || req.IDUsuario == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Elige a un compañero")
	}
	if err := h.repository.Invite(usuario(c), req.IDUsuario); err != nil {
		return errorLab(c, err)
	}
	return c.SendString("success")
}

func (h *LaboratorioHandler) Answer(c fiber.Ctx) error {
	var req struct {
		Aceptar bool `json:"aceptar"`
	}
	_ = c.Bind().Body(&req)

	if err := h.repository.Answer(c.Params("id"), usuario(c), req.Aceptar); err != nil {
		return errorLab(c, err)
	}
	return c.SendString("success")
}

// ---------------------------------------------------------------------
// Monitor (solo admin)
// ---------------------------------------------------------------------

func (h *LaboratorioHandler) Monitor(c fiber.Ctx) error {
	return c.JSON(h.repository.All())
}

func (h *LaboratorioHandler) Close(c fiber.Ctx) error {
	lab := h.repository.ByID(c.Params("id"))
	if lab == nil {
		return c.Status(fiber.StatusNotFound).SendString("Ese laboratorio ya no existe")
	}
	if err := h.laboratorios.Destroy(c.Context(), lab); err != nil {
		return errorLab(c, err)
	}
	return c.SendString("success")
}
