package handler

import (
	"errors"
	"log"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

// PlantillaHandler: el profesor despliega topologías en SU laboratorio
type PlantillaHandler struct {
	despliegues *service.Despliegues
}

func NewPlantillaHandler(s *service.Despliegues) *PlantillaHandler {
	return &PlantillaHandler{despliegues: s}
}

// Desplegar arranca el despliegue en segundo plano; el avance se consulta con Estado
func (h *PlantillaHandler) Desplegar(c fiber.Ctx) error {
	var req domain.SolicitudDespliegue
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos del despliegue inválidos")
	}

	err := h.despliegues.DesplegarPlantilla(usuario(c), &req)
	if err == nil {
		return c.SendString("success")
	}

	var ev *service.ErrValidacion
	switch {
	case errors.As(err, &ev):
		return c.Status(fiber.StatusBadRequest).SendString(ev.Texto)
	case errors.Is(err, repository.ErrSinLaboratorio), errors.Is(err, repository.ErrNoEsDueno),
		errors.Is(err, service.ErrLabNoCorriendo), errors.Is(err, service.ErrProyectoExiste),
		errors.Is(err, repository.ErrDespliegueEnCurso):
		return c.Status(fiber.StatusConflict).SendString(err.Error())
	}
	log.Printf("[DESPLIEGUE] %v", err)
	return c.Status(fiber.StatusInternalServerError).SendString("No se pudo iniciar el despliegue: " + err.Error())
}

// Estado: avance y resultado del último despliegue del laboratorio
func (h *PlantillaHandler) Estado(c fiber.Ctx) error {
	return c.JSON(h.despliegues.Estado(usuario(c)))
}
