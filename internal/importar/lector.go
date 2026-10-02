// Package importar: carga masiva de usuarios desde CSV o Excel (.xlsx).
// Lee el archivo FILA POR FILA (streaming): un archivo de 100 MB no se
// carga completo en memoria.
package importar

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

// Lector entrega una fila a la vez. Regresa io.EOF al terminar.
type Lector interface {
	Siguiente() ([]string, error)
	Progreso() float64 // 0..1 según bytes leídos
	Cerrar() error
}

// contador cuenta los bytes leídos para calcular el porcentaje
type contador struct {
	r     io.Reader
	leido int64
}

func (c *contador) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.leido += int64(n)
	return n, err
}

// Abrir elige el lector según la extensión
func Abrir(ruta string) (Lector, error) {
	switch strings.ToLower(path.Ext(ruta)) {
	case ".csv", ".txt":
		return abrirCSV(ruta)
	case ".xlsx":
		return abrirXLSX(ruta)
	default:
		return nil, errors.New("formato no soportado (usa .csv o .xlsx)")
	}
}

// =====================================================================
// CSV
// =====================================================================

type lectorCSV struct {
	f     *os.File
	c     *contador
	total int64
	r     *csv.Reader
}

func abrirCSV(ruta string) (*lectorCSV, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	info, _ := f.Stat()

	c := &contador{r: f}
	br := bufio.NewReaderSize(c, 64*1024)

	// Quitar BOM de UTF-8 (Excel lo agrega al "Guardar como CSV UTF-8")
	if b, _ := br.Peek(3); bytes.Equal(b, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}

	// Detectar separador: Excel en español suele usar ';'
	sep := ','
	if linea, _ := br.Peek(4096); bytes.Count(linea, []byte(";")) > bytes.Count(linea, []byte(",")) {
		sep = ';'
	}

	r := csv.NewReader(br)
	r.Comma = sep
	r.FieldsPerRecord = -1 // filas con distinto número de columnas no truenan
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.ReuseRecord = true // reutiliza el mismo slice en cada fila: menos memoria

	return &lectorCSV{f: f, c: c, total: info.Size(), r: r}, nil
}

func (l *lectorCSV) Siguiente() ([]string, error) { return l.r.Read() }
func (l *lectorCSV) Cerrar() error                { return l.f.Close() }
func (l *lectorCSV) Progreso() float64 {
	if l.total <= 0 {
		return 0
	}
	return min(1, float64(l.c.leido)/float64(l.total))
}

// =====================================================================
// XLSX (es un .zip con XML adentro; se lee la PRIMERA hoja en streaming)
// =====================================================================

type lectorXLSX struct {
	zr      *zip.ReadCloser
	hoja    io.ReadCloser
	c       *contador
	total   int64
	dec     *xml.Decoder
	cadenas []string // sharedStrings: textos que Excel guarda una sola vez
	fila    []string // se reutiliza en cada fila: menos memoria
}

func abrirXLSX(ruta string) (*lectorXLSX, error) {
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		return nil, fmt.Errorf("el archivo no es un .xlsx válido: %w", err)
	}

	archivos := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		archivos[f.Name] = f
	}

	l := &lectorXLSX{zr: zr}

	if f, ok := archivos["xl/sharedStrings.xml"]; ok {
		if l.cadenas, err = leerCadenas(f); err != nil {
			zr.Close()
			return nil, err
		}
	}

	hoja := primeraHoja(archivos)
	if hoja == nil {
		zr.Close()
		return nil, errors.New("el .xlsx no tiene hojas")
	}
	rc, err := hoja.Open()
	if err != nil {
		zr.Close()
		return nil, err
	}
	l.hoja = rc
	l.c = &contador{r: rc}
	l.total = int64(hoja.UncompressedSize64)
	l.dec = xml.NewDecoder(bufio.NewReaderSize(l.c, 64*1024))
	return l, nil
}

// primeraHoja busca la hoja 1 del libro (normalmente xl/worksheets/sheet1.xml)
func primeraHoja(archivos map[string]*zip.File) *zip.File {
	if f, ok := archivos["xl/worksheets/sheet1.xml"]; ok {
		return f
	}
	var elegido *zip.File
	for nombre, f := range archivos {
		if strings.HasPrefix(nombre, "xl/worksheets/") && strings.HasSuffix(nombre, ".xml") {
			if elegido == nil || nombre < elegido.Name {
				elegido = f
			}
		}
	}
	return elegido
}

// leerCadenas carga la tabla de textos compartidos (<si><t>texto</t></si>)
func leerCadenas(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	dec := xml.NewDecoder(bufio.NewReaderSize(rc, 64*1024))
	var (
		cadenas []string
		actual  strings.Builder
		dentroT bool
		dentroR bool // <rPh> = texto fonético japonés, se ignora
	)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return cadenas, nil
		}
		if err != nil {
			return nil, fmt.Errorf("leyendo sharedStrings: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				actual.Reset()
			case "t":
				dentroT = true
			case "rPh":
				dentroR = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "si":
				cadenas = append(cadenas, actual.String())
			case "t":
				dentroT = false
			case "rPh":
				dentroR = false
			}
		case xml.CharData:
			if dentroT && !dentroR {
				actual.Write(t)
			}
		}
	}
}

// columna convierte "C12" -> 2 (índice desde 0)
func columna(ref string) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
	}
	return n - 1
}

// numero limpia valores numéricos de Excel: "2023371001.0" -> "2023371001"
func numero(v string) string {
	if f, err := strconv.ParseFloat(v, 64); err == nil && f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return v
}

func (l *lectorXLSX) Siguiente() ([]string, error) {
	var (
		enFila  bool
		col     int
		tipo    string
		valor   strings.Builder
		enValor bool
	)
	l.fila = l.fila[:0]

	for {
		tok, err := l.dec.Token()
		if err == io.EOF {
			return nil, io.EOF
		}
		if err != nil {
			return nil, fmt.Errorf("leyendo hoja: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				enFila = true
				l.fila = l.fila[:0]
			case "c":
				tipo, col = "", len(l.fila)
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "t":
						tipo = a.Value
					case "r":
						col = columna(a.Value)
					}
				}
				valor.Reset()
			case "v", "t":
				enValor = true
			}

		case xml.CharData:
			if enValor {
				valor.Write(t)
			}

		case xml.EndElement:
			switch t.Name.Local {
			case "v", "t":
				enValor = false
			case "c":
				if !enFila || col < 0 {
					continue
				}
				v := valor.String()
				switch tipo {
				case "s": // índice a sharedStrings
					if i, err := strconv.Atoi(v); err == nil && i >= 0 && i < len(l.cadenas) {
						v = l.cadenas[i]
					}
				case "", "n":
					v = numero(v)
				}
				// Rellenar celdas vacías que Excel no escribe
				for len(l.fila) < col {
					l.fila = append(l.fila, "")
				}
				l.fila = append(l.fila, v)
			case "row":
				return l.fila, nil
			}
		}
	}
}

func (l *lectorXLSX) Progreso() float64 {
	if l.total <= 0 {
		return 0
	}
	return min(1, float64(l.c.leido)/float64(l.total))
}

func (l *lectorXLSX) Cerrar() error {
	l.hoja.Close()
	return l.zr.Close()
}
