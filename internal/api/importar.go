package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"raccoon_lab/internal/importar"
)

// Límite de subida: 100 MB de archivo + margen para el multipart
const maxArchivo = 100 << 20

// SubirImportacion: POST /api/admin/importar/{tipo}   (form-data: archivo)
// Guarda el archivo en disco mientras llega (sin cargarlo en memoria),
// responde de inmediato con el id y procesa en segundo plano.
func (a *App) SubirImportacion(w http.ResponseWriter, r *http.Request) {
	tipo := r.PathValue("tipo")
	if _, ok := importar.Tipos[tipo]; !ok {
		responderError(w, http.StatusBadRequest, "Tipo inválido: usa alumnos, maestros o admins")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxArchivo+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		responderError(w, http.StatusBadRequest, "Se esperaba un formulario con el campo 'archivo'")
		return
	}

	for {
		parte, err := mr.NextPart()
		if err == io.EOF {
			responderError(w, http.StatusBadRequest, "No se recibió ningún archivo")
			return
		}
		if err != nil {
			responderError(w, http.StatusBadRequest, "Error leyendo la subida")
			return
		}
		if parte.FormName() != "archivo" {
			continue
		}

		nombre := filepath.Base(parte.FileName())
		ext := strings.ToLower(filepath.Ext(nombre))
		if ext != ".csv" && ext != ".xlsx" {
			responderError(w, http.StatusBadRequest, "Formato no soportado: sube un .csv o .xlsx (si es .xls, guárdalo como .xlsx)")
			return
		}

		t, err := a.Importador.Reservar(tipo, nombre)
		if errors.Is(err, importar.ErrOcupado) {
			responderError(w, http.StatusConflict, err.Error())
			return
		}

		destino, err := os.Create(a.Importador.Ruta(t))
		if err != nil {
			a.Importador.Cancelar(t, "no se pudo guardar el archivo")
			responderError(w, http.StatusInternalServerError, "No se pudo guardar el archivo")
			return
		}
		n, err := io.Copy(destino, io.LimitReader(parte, maxArchivo+1))
		destino.Close()

		switch {
		case err != nil:
			a.Importador.Cancelar(t, "subida interrumpida")
			responderError(w, http.StatusBadRequest, "La subida se interrumpió")
			return
		case n > maxArchivo:
			a.Importador.Cancelar(t, "archivo mayor a 100 MB")
			responderError(w, http.StatusRequestEntityTooLarge, "El archivo pasa de 100 MB")
			return
		case n == 0:
			a.Importador.Cancelar(t, "archivo vacío")
			responderError(w, http.StatusBadRequest, "El archivo está vacío")
			return
		}

		log.Printf("[IMPORTAR] %s subió %s (%s, %.1f MB)", sesionDe(r).Matricula, nombre, tipo, float64(n)/(1<<20))
		go a.Importador.Procesar(t)
		responderJSON(w, http.StatusAccepted, t.Copia())
		return
	}
}

// EstadoImportacion: GET /api/admin/importar/{id}  -> progreso en vivo
func (a *App) EstadoImportacion(w http.ResponseWriter, r *http.Request) {
	t := a.Importador.Obtener(r.PathValue("id"))
	if t == nil {
		responderError(w, http.StatusNotFound, "Importación no encontrada")
		return
	}
	responderJSON(w, http.StatusOK, t)
}

// PlantillaImportacion: GET /api/admin/importar/plantilla/{tipo} -> CSV de ejemplo
func (a *App) PlantillaImportacion(w http.ResponseWriter, r *http.Request) {
	tipo, ok := importar.Tipos[r.PathValue("tipo")]
	if !ok {
		responderError(w, http.StatusBadRequest, "Tipo inválido: usa alumnos, maestros o admins")
		return
	}
	ejemplo := map[string]string{
		"alumnos":  "2023371001,Juan Pérez López,Cambiar123,IRD-71",
		"maestros": "mgarcia,María García Ruiz,Cambiar123",
		"admins":   "jlopez,José López Díaz,Cambiar123",
	}[tipo.Nombre]

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="plantilla_%s.csv"`, tipo.Nombre))
	// BOM para que Excel abra bien los acentos
	fmt.Fprintf(w, "\xEF\xBB\xBF%s\r\n%s\r\n", strings.Join(tipo.Columnas, ","), ejemplo)
}
