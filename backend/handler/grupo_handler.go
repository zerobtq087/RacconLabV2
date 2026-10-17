package handler

import (
	"errors"
	"strings"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type GrupoHandler struct {
	repository *repository.GrupoRepository
}

func NewGrupoHandler(r *repository.GrupoRepository) *GrupoHandler {
	return &GrupoHandler{repository: r}
}

// GetAll: el admin ve todos; el maestro solo los suyos
func (h *GrupoHandler) GetAll(c fiber.Ctx) error {
	if rol, _ := c.Locals("rol").(string); rol == "ADMIN" {
		return c.JSON(h.repository.GetAll())
	}
	userID, _ := c.Locals("userID").(string)
	return c.JSON(h.repository.GetByDocente(userID))
}

func (h *GrupoHandler) Alumnos(c fiber.Ctx) error {
	id := c.Params("id")
	rol, _ := c.Locals("rol").(string)
	userID, _ := c.Locals("userID").(string)

	if rol != "ADMIN" && !h.repository.IsDocente(id, userID) {
		return c.Status(fiber.StatusForbidden).SendString("Ese grupo no es tuyo")
	}
	return c.JSON(h.repository.Alumnos(id))
}

func errorGrupo(c fiber.Ctx, err error) error {
	if errors.Is(err, repository.ErrGrupoOcupado) || errors.Is(err, repository.ErrGrupoAlumnos) {
		return c.Status(fiber.StatusConflict).SendString(err.Error())
	}
	return c.Status(fiber.StatusInternalServerError).SendString("No se pudo guardar el grupo")
}

func (h *GrupoHandler) Create(c fiber.Ctx) error {
	var g domain.Grupo
	if err := c.Bind().Body(&g); err != nil || strings.TrimSpace(g.Codigo) == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Escribe el código del grupo")
	}

	g.ID = uuid.NewString()
	if err := h.repository.Create(&g); err != nil {
		return errorGrupo(c, err)
	}
	return c.SendString(g.ID)
}

func (h *GrupoHandler) Update(c fiber.Ctx) error {
	var g domain.Grupo
	if err := c.Bind().Body(&g); err != nil || strings.TrimSpace(g.Codigo) == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Escribe el código del grupo")
	}

	g.ID = c.Params("id")
	if err := h.repository.Update(&g); err != nil {
		return errorGrupo(c, err)
	}
	return c.SendString("success")
}

func (h *GrupoHandler) Delete(c fiber.Ctx) error {
	if err := h.repository.Delete(c.Params("id")); err != nil {
		return errorGrupo(c, err)
	}
	return c.SendString("success")
}

func (h *GrupoHandler) SetDocentes(c fiber.Ctx) error {
	var req struct {
		Docentes []string `json:"docentes"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}

	if err := h.repository.SetDocentes(c.Params("id"), req.Docentes); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudieron asignar los maestros")
	}
	return c.SendString("success")
}

func (h *GrupoHandler) Carreras(c fiber.Ctx) error {
	return c.JSON(h.repository.Carreras())
}
