package service

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"strings"
)

// Filas de ejemplo de cada plantilla (en el orden de columnasImport)
var ejemplosImport = map[string][][]string{
	"alumnos": {
		{"2026371001", "Ana López Pérez", "Cambiar.2026", "IRD-71", "Ingeniería en Redes Inteligentes y Ciberseguridad"},
		{"2026371002", "Luis Hernández Ruiz", "Cambiar.2026", "IRD-71", "Ingeniería en Redes Inteligentes y Ciberseguridad"},
	},
	"maestros": {
		{"mgarcia", "María García Soto", "Cambiar.2026"},
		{"jramirez", "Jorge Ramírez León", "Cambiar.2026"},
	},
	"admins": {
		{"soporte", "Soporte Técnico", "Cambiar.2026"},
	},
	"general": {
		{"2026371003", "Carla Méndez Ortiz", "Cambiar.2026", "ALUMNO", "IRD-72", "Ingeniería en Redes Inteligentes y Ciberseguridad"},
		{"lflores", "Laura Flores Vega", "Cambiar.2026", "PROFESOR", "", ""},
	},
}

// PlantillaImport arma el archivo de ejemplo para descargar.
// Regresa el contenido, el nombre del archivo y su Content-Type.
func PlantillaImport(tipo, formato string) ([]byte, string, string, error) {
	columnas, ok := columnasImport[tipo]
	if !ok {
		return nil, "", "", invalido("Tipo inválido (alumnos, maestros, admins o general)")
	}
	filas := append([][]string{columnas}, ejemplosImport[tipo]...)
	nombre := "plantilla_" + tipo

	switch formato {
	case "csv":
		datos, err := plantillaCSV(filas)
		return datos, nombre + ".csv", "text/csv; charset=utf-8", err
	case "xlsx":
		datos, err := plantillaXLSX(filas)
		return datos, nombre + ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	}
	return nil, "", "", invalido("Formato inválido (csv o xlsx)")
}

// plantillaCSV: UTF-8 con BOM (así Excel muestra bien "contraseña" y los acentos)
func plantillaCSV(filas [][]string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // BOM de UTF-8
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.WriteAll(filas); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// plantillaXLSX escribe un .xlsx mínimo: es un .zip con 5 XML. Todas las celdas
// van como texto (inlineStr) para que Excel no convierta las matrículas en números.
func plantillaXLSX(filas [][]string) ([]byte, error) {
	return escribirXLSX("usuarios", filas, nil)
}

// escribirXLSX arma un .xlsx de una hoja; las columnas en "numericas" se
// escriben como número (para que Excel pueda sacar promedios)
func escribirXLSX(nombreHoja string, filas [][]string, numericas map[int]bool) ([]byte, error) {
	const (
		cabecera = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
		nsMain   = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
		nsRels   = "http://schemas.openxmlformats.org/package/2006/relationships"
		nsDoc    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	)

	var hoja strings.Builder
	hoja.WriteString(cabecera + `<worksheet xmlns="` + nsMain + `">`)
	hoja.WriteString(`<cols><col min="1" max="6" width="24" customWidth="1"/></cols><sheetData>`)
	for i, fila := range filas {
		fmt.Fprintf(&hoja, `<row r="%d">`, i+1)
		for j, valor := range fila {
			if numericas[j] && i > 0 && valor != "" {
				fmt.Fprintf(&hoja, `<c r="%c%d"><v>%s</v></c>`, 'A'+j, i+1, valor)
				continue
			}
			fmt.Fprintf(&hoja, `<c r="%c%d" t="inlineStr"><is><t>`, 'A'+j, i+1)
			if err := xml.EscapeText(&hoja, []byte(valor)); err != nil {
				return nil, err
			}
			hoja.WriteString(`</t></is></c>`)
		}
		hoja.WriteString(`</row>`)
	}
	hoja.WriteString(`</sheetData></worksheet>`)

	archivos := []struct{ nombre, contenido string }{
		{"[Content_Types].xml", cabecera + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`</Types>`},
		{"_rels/.rels", cabecera + `<Relationships xmlns="` + nsRels + `">` +
			`<Relationship Id="rId1" Type="` + nsDoc + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", cabecera + `<workbook xmlns="` + nsMain + `" xmlns:r="` + nsDoc + `">` +
			`<sheets><sheet name="` + nombreHoja + `" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", cabecera + `<Relationships xmlns="` + nsRels + `">` +
			`<Relationship Id="rId1" Type="` + nsDoc + `/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", hoja.String()},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, a := range archivos {
		w, err := zw.Create(a.nombre)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(a.contenido)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
