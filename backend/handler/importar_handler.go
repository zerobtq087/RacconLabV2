package handler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

type ImportarHandler struct {
	repository *repository.ImportacionRepository
	importador *service.Importador
}

func NewImportarHandler(r *repository.ImportacionRepository, im *service.Importador) *ImportarHandler {
	return &ImportarHandler{repository: r, importador: im}
}

// general: un solo archivo con todos; cada fila trae su tipo (ALUMNO, PROFESOR o ADMIN)
var tiposValidos = map[string]bool{"alumnos": true, "maestros": true, "admins": true, "general": true}

// Upload recibe el archivo, lo guarda y lo procesa en segundo plano
func (h *ImportarHandler) Upload(c fiber.Ctx) error {
	tipo := c.FormValue("tipo")
	if !tiposValidos[tipo] {
		return c.Status(fiber.StatusBadRequest).SendString("Tipo inválido (alumnos, maestros, admins o general)")
	}

	archivo, err := c.FormFile("archivo")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Falta el archivo")
	}
	ext := strings.ToLower(filepath.Ext(archivo.Filename))
	if ext != ".csv" && ext != ".xlsx" {
		return c.Status(fiber.StatusBadRequest).SendString("Solo se aceptan archivos .csv o .xlsx")
	}

	userID, _ := c.Locals("userID").(string)
	id, err := h.repository.Reserve(tipo, archivo.Filename, userID)
	if err != nil {
		if errors.Is(err, repository.ErrImportacionOcupada) {
			return c.Status(fiber.StatusConflict).SendString(err.Error())
		}
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo iniciar la importación")
	}

	ruta := filepath.Join(os.TempDir(), "raccoon-import-"+id+ext)
	if err := c.SaveFile(archivo, ruta); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo guardar el archivo")
	}

	go h.importador.Process(id, tipo, archivo.Filename, ruta)

	return c.JSON(fiber.Map{"id": id})
}

// Plantilla descarga el archivo de ejemplo: /api/importar/plantilla?tipo=alumnos&formato=xlsx
func (h *ImportarHandler) Plantilla(c fiber.Ctx) error {
	datos, nombre, tipoContenido, err := service.PlantillaImport(c.Query("tipo"), c.Query("formato"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	c.Set(fiber.HeaderContentType, tipoContenido)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+nombre+`"`)
	return c.Send(datos)
}

// Status regresa el avance (lo pregunta el frontend cada segundo)
func (h *ImportarHandler) Status(c fiber.Ctx) error {
	imp := h.repository.GetByID(c.Params("id"))
	if imp == nil {
		return c.Status(fiber.StatusNotFound).SendString("No existe esa importación")
	}
	return c.JSON(imp)
}
