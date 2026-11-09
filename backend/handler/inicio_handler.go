package handler

import (
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

// InicioHandler: el tablero de la página de Inicio
type InicioHandler struct {
	inicio *service.Inicio
}

func NewInicioHandler(s *service.Inicio) *InicioHandler {
	return &InicioHandler{inicio: s}
}

// Tablero: solo lo que puede ver el rol con el que se entró (c.Locals("rol"))
func (h *InicioHandler) Tablero(c fiber.Ctx) error {
	rol, _ := c.Locals("rol").(string)
	return c.JSON(h.inicio.Tablero(usuario(c), rol))
}
