package handler

import (
	"errors"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

// CalculadoraHandler: calculadora de redes (alumno, profesor y admin).
// Todo el cálculo está en service/calculadora.go; aquí solo se lee y responde.
type CalculadoraHandler struct{}

func NewCalculadoraHandler() *CalculadoraHandler {
	return &CalculadoraHandler{}
}

// responder: JSON si salió bien; 400 con el texto si los datos están mal
func responder(c fiber.Ctx, resultado any, err error) error {
	var ev *service.ErrValidacion
	if errors.As(err, &ev) {
		return c.Status(fiber.StatusBadRequest).SendString(ev.Texto)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo calcular")
	}
	return c.JSON(resultado)
}

func (h *CalculadoraHandler) Subred(c fiber.Ctx) error {
	var req struct {
		IP      string `json:"ip"`
		Mascara string `json:"mascara"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	r, err := service.Subred(req.IP, req.Mascara)
	return responder(c, r, err)
}

func (h *CalculadoraHandler) Dividir(c fiber.Ctx) error {
	var req domain.SolicitudDivision
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	r, err := service.Dividir(&req)
	return responder(c, r, err)
}

func (h *CalculadoraHandler) VLSM(c fiber.Ctx) error {
	var req domain.SolicitudVLSM
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	r, err := service.VLSM(&req)
	return responder(c, r, err)
}

func (h *CalculadoraHandler) Enlaces(c fiber.Ctx) error {
	var req domain.SolicitudEnlaces
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	r, err := service.Enlaces(&req)
	return responder(c, r, err)
}

func (h *CalculadoraHandler) Sumarizar(c fiber.Ctx) error {
	var req domain.SolicitudSumarizacion
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	r, err := service.Sumarizar(&req)
	return responder(c, r, err)
}

func (h *CalculadoraHandler) Convertir(c fiber.Ctx) error {
	var req domain.SolicitudConversion
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	r, err := service.Convertir(&req)
	return responder(c, r, err)
}
