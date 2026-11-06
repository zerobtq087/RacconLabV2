package handler

import (
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

// SupervisionHandler: el docente/admin revisa los labs de sus alumnos
type SupervisionHandler struct {
	supervision *service.Supervision
}

func NewSupervisionHandler(s *service.Supervision) *SupervisionHandler {
	return &SupervisionHandler{supervision: s}
}

// Alumnos: /api/supervision/grupos/:grupo
func (h *SupervisionHandler) Alumnos(c fiber.Ctx) error {
	lista, err := h.supervision.Alumnos(usuario(c), c.Params("grupo"))
	if err != nil {
		return responder(c, err, "Supervisión")
	}
	return c.JSON(lista)
}

func (h *SupervisionHandler) Proyectos(c fiber.Ctx) error {
	lista, err := h.supervision.Proyectos(usuario(c), c.Params("lab"))
	if err != nil {
		return responder(c, err, "No se pudieron leer los proyectos")
	}
	return c.JSON(lista)
}

// Nodos: ?actual=1 lee la running-config por consola; si no, la guardada
func (h *SupervisionHandler) Nodos(c fiber.Ctx) error {
	lista, err := h.supervision.Nodos(usuario(c), c.Params("lab"), c.Params("proyecto"), c.Query("actual") == "1")
	if err != nil {
		return responder(c, err, "No se pudieron leer los nodos")
	}
	return c.JSON(lista)
}

// Descargar: ?detener=1 detiene el proyecto si tiene nodos encendidos
func (h *SupervisionHandler) Descargar(c fiber.Ctx) error {
	zip, nombre, err := h.supervision.Descargar(usuario(c), c.Params("lab"), c.Params("proyecto"), c.Query("detener") == "1")
	if err != nil {
		return responder(c, err, "No se pudo descargar")
	}
	return adjunto(c, zip, nombre, "application/zip")
}

func (h *SupervisionHandler) Copiar(c fiber.Ctx) error {
	nombre, err := h.supervision.CopiarAMiLab(usuario(c), c.Params("lab"), c.Params("proyecto"), c.Query("detener") == "1")
	if err != nil {
		return responder(c, err, "No se pudo copiar a tu laboratorio")
	}
	return c.JSON(fiber.Map{"proyecto": nombre})
}

// Consolas: abre el firewall del lab para la IP del docente por 30 min
func (h *SupervisionHandler) Consolas(c fiber.Ctx) error {
	puerto, hasta, err := h.supervision.Supervisar(usuario(c), c.Params("lab"))
	if err != nil {
		return responder(c, err, "No se pudieron abrir las consolas")
	}
	return c.JSON(fiber.Map{"puerto": puerto, "hasta": hasta})
}

func (h *SupervisionHandler) CerrarConsolas(c fiber.Ctx) error {
	h.supervision.DejarDeSupervisar(usuario(c), c.Params("lab"))
	return c.SendString("success")
}

func (h *SupervisionHandler) Bitacora(c fiber.Ctx) error {
	return c.JSON(h.supervision.Bitacora(usuario(c)))
}
