package handler

import (
	"errors"
	"log"
	"net/url"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

// PracticaHandler: prácticas con calificación (profesor/admin y alumno)
type PracticaHandler struct {
	practicas *service.Practicas
}

func NewPracticaHandler(s *service.Practicas) *PracticaHandler {
	return &PracticaHandler{practicas: s}
}

// responder: "success" o el error con su status (como fiber3)
func responder(c fiber.Ctx, err error, contexto string) error {
	if err == nil {
		return c.SendString("success")
	}
	var ev *service.ErrValidacion
	var otros *service.ErrOtrosAbiertos
	switch {
	case errors.As(err, &ev):
		return c.Status(fiber.StatusBadRequest).SendString(ev.Texto)
	case errors.As(err, &otros):
		// 409 con la lista: la página pregunta si se detienen
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"mensaje": otros.Error(), "proyectos": otros.Proyectos})
	case errors.Is(err, service.ErrSinPermiso):
		return c.Status(fiber.StatusForbidden).SendString(err.Error())
	case errors.Is(err, repository.ErrPracticaNoEsta):
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	case errors.Is(err, repository.ErrSinLaboratorio), errors.Is(err, service.ErrLabNoCorriendo),
		errors.Is(err, repository.ErrRevisionEnCurso), errors.Is(err, service.ErrYaEntregaste),
		errors.Is(err, service.ErrPracticaNoActiva), errors.Is(err, service.ErrConEntregas),
		errors.Is(err, service.ErrAbrirPrimero), errors.Is(err, service.ErrHayQueDetener):
		return c.Status(fiber.StatusConflict).SendString(err.Error())
	}
	log.Printf("[%s] %v", contexto, err)
	return c.Status(fiber.StatusInternalServerError).SendString(contexto + ": " + err.Error())
}

// adjunto manda un archivo para descargar (el nombre puede llevar acentos)
func adjunto(c fiber.Ctx, datos []byte, nombre, tipo string) error {
	c.Set(fiber.HeaderContentType, tipo)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename*=UTF-8''`+url.PathEscape(nombre))
	return c.Send(datos)
}

// ---------------------------------------------------------------------
// Profesor / admin
// ---------------------------------------------------------------------

func (h *PracticaHandler) Listar(c fiber.Ctx) error {
	return c.JSON(h.practicas.Listar(usuario(c)))
}

// Proyectos: los proyectos del lab del profesor (para elegir de cuál sale la práctica)
func (h *PracticaHandler) Proyectos(c fiber.Ctx) error {
	lista, err := h.practicas.ProyectosDeMiLab(usuario(c))
	if err != nil {
		return responder(c, err, "No se pudieron leer tus proyectos")
	}
	return c.JSON(lista)
}

func (h *PracticaHandler) Crear(c fiber.Ctx) error {
	var req domain.SolicitudPractica
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos de la práctica inválidos")
	}
	id, err := h.practicas.Crear(usuario(c), &req)
	if err != nil {
		return responder(c, err, "No se pudo crear la práctica")
	}
	return c.JSON(fiber.Map{"id": id})
}

func (h *PracticaHandler) Detalle(c fiber.Ctx) error {
	p, err := h.practicas.Detalle(usuario(c), c.Params("id"))
	if err != nil {
		return responder(c, err, "Práctica")
	}
	return c.JSON(p)
}

func (h *PracticaHandler) Editar(c fiber.Ctx) error {
	var req domain.SolicitudPractica
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos de la práctica inválidos")
	}
	return responder(c, h.practicas.Editar(usuario(c), c.Params("id"), &req), "No se pudo guardar")
}

func (h *PracticaHandler) Criterios(c fiber.Ctx) error {
	var req []domain.Criterio
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Criterios inválidos")
	}
	return responder(c, h.practicas.GuardarCriterios(usuario(c), c.Params("id"), req), "No se pudieron guardar los criterios")
}

func (h *PracticaHandler) Activar(c fiber.Ctx) error {
	return responder(c, h.practicas.Activar(usuario(c), c.Params("id")), "No se pudo activar")
}

func (h *PracticaHandler) Cerrar(c fiber.Ctx) error {
	return responder(c, h.practicas.Cerrar(usuario(c), c.Params("id")), "No se pudo cerrar")
}

func (h *PracticaHandler) Borrar(c fiber.Ctx) error {
	return responder(c, h.practicas.Borrar(usuario(c), c.Params("id")), "No se pudo borrar")
}

// Probar revisa los criterios contra el proyecto original del profesor
func (h *PracticaHandler) Probar(c fiber.Ctx) error {
	return responder(c, h.practicas.Probar(usuario(c), c.Params("id")), "No se pudo iniciar la prueba")
}

func (h *PracticaHandler) Prueba(c fiber.Ctx) error {
	p, err := h.practicas.Prueba(usuario(c), c.Params("id"))
	if err != nil {
		return responder(c, err, "Prueba")
	}
	return c.JSON(p)
}

func (h *PracticaHandler) Entregas(c fiber.Ctx) error {
	filas, err := h.practicas.Entregas(usuario(c), c.Params("id"))
	if err != nil {
		return responder(c, err, "Entregas")
	}
	return c.JSON(filas)
}

func (h *PracticaHandler) Excel(c fiber.Ctx) error {
	datos, nombre, err := h.practicas.Excel(usuario(c), c.Params("id"))
	if err != nil {
		return responder(c, err, "No se pudo generar el Excel")
	}
	return adjunto(c, datos, nombre, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func (h *PracticaHandler) Entrega(c fiber.Ctx) error {
	d, err := h.practicas.DetalleEntrega(usuario(c), c.Params("id"), c.Params("alumno"))
	if err != nil {
		return responder(c, err, "Entrega")
	}
	return c.JSON(d)
}

func (h *PracticaHandler) ProyectoEntrega(c fiber.Ctx) error {
	zip, nombre, err := h.practicas.ProyectoEntrega(usuario(c), c.Params("id"), c.Params("alumno"))
	if err != nil {
		return responder(c, err, "No se pudo descargar")
	}
	return adjunto(c, zip, nombre, "application/zip")
}

func (h *PracticaHandler) AbrirEntrega(c fiber.Ctx) error {
	nombre, err := h.practicas.AbrirEntregaEnMiLab(usuario(c), c.Params("id"), c.Params("alumno"))
	if err != nil {
		return responder(c, err, "No se pudo copiar a tu laboratorio")
	}
	return c.JSON(fiber.Map{"proyecto": nombre})
}

// ---------------------------------------------------------------------
// Alumno
// ---------------------------------------------------------------------

func (h *PracticaHandler) MisPracticas(c fiber.Ctx) error {
	return c.JSON(h.practicas.MisPracticas(usuario(c)))
}

func (h *PracticaHandler) Abrir(c fiber.Ctx) error {
	var req domain.SolicitudAbrir
	_ = c.Bind().Body(&req) // sin cuerpo = sin opciones
	nombre, err := h.practicas.Abrir(usuario(c), c.Params("id"), &req)
	if err != nil {
		return responder(c, err, "No se pudo abrir la práctica")
	}
	return c.JSON(fiber.Map{"proyecto": nombre})
}

func (h *PracticaHandler) Entregar(c fiber.Ctx) error {
	return responder(c, h.practicas.Entregar(usuario(c), c.Params("id")), "No se pudo entregar")
}
