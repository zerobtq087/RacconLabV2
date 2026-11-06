package service

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cada plantilla descargable debe poder importarse tal cual: el importador
// reconoce sus encabezados y sus filas de ejemplo son válidas.
func TestPlantillasSeImportan(t *testing.T) {
	esperado := map[string][]string{ // tipo de usuario de cada fila de ejemplo
		"alumnos":  {"alumnos", "alumnos"},
		"maestros": {"maestros", "maestros"},
		"admins":   {"admins"},
		"general":  {"alumnos", "maestros"},
	}
	for tipo, tipos := range esperado {
		for _, formato := range []string{"csv", "xlsx"} {
			t.Run(tipo+"."+formato, func(t *testing.T) {
				datos, nombre, _, err := PlantillaImport(tipo, formato)
				if err != nil {
					t.Fatal(err)
				}
				ruta := filepath.Join(t.TempDir(), nombre)
				if err := os.WriteFile(ruta, datos, 0o600); err != nil {
					t.Fatal(err)
				}
				l, err := Abrir(ruta)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Cerrar()

				enc, err := l.Siguiente()
				if err != nil {
					t.Fatal(err)
				}
				idx, ok := detectarEncabezados(append([]string(nil), enc...), tipo)
				if !ok {
					t.Fatalf("el importador no reconoce los encabezados %v", enc)
				}
				for n, tipoFila := range tipos {
					reg, err := l.Siguiente()
					if err != nil {
						t.Fatalf("fila %d: %v", n+2, err)
					}
					f, msg := validar(reg, idx, tipo, n+2)
					if msg != "" {
						t.Fatalf("fila %d de ejemplo inválida: %s", n+2, msg)
					}
					if f.Tipo != tipoFila {
						t.Errorf("fila %d: tipo %s, se esperaba %s", n+2, f.Tipo, tipoFila)
					}
				}
				if _, err := l.Siguiente(); err != io.EOF {
					t.Errorf("sobran filas en la plantilla")
				}
			})
		}
	}
}

func TestEncabezadosSinonimos(t *testing.T) {
	// Encabezados viejos o en otro orden también sirven
	idx, ok := detectarEncabezados([]string{"Rol", "Clave", "Nombre completo", "Contraseña", "Carrera", "Grupo"}, "general")
	if !ok || idx[cTipo] != 0 || idx[cID] != 1 || idx[cPassword] != 3 || idx[cGrupo] != 5 {
		t.Fatalf("sinónimos no reconocidos: %v %v", idx, ok)
	}
	if _, ok := detectarEncabezados([]string{"matricula", "nombre", "contraseña"}, "general"); ok {
		t.Error("el archivo general sin columna tipo no debe aceptarse como encabezado")
	}
	if _, ok := detectarEncabezados([]string{"matricula", "nombre", "contraseña"}, "alumnos"); ok {
		t.Error("el archivo de alumnos sin grupo no debe aceptarse como encabezado")
	}
}

func TestFilasGeneral(t *testing.T) {
	idx := ordenPlantilla("general") // matricula, nombre, contraseña, tipo, grupo, carrera
	casos := []struct {
		fila     []string
		tipo     string
		contiene string // "" = válida
	}{
		{[]string{"2026001", "Ana", "x1", "alumno", "ird-71", ""}, "alumnos", ""},
		{[]string{"prof1", "Beto", "x1", "Maestro", "IRD-71", "Redes"}, "maestros", ""},
		{[]string{"adm1", "Caro", "x1", "ADMINISTRADOR", "", ""}, "admins", ""},
		{[]string{"2026002", "Dani", "x1", "ALUMNO", "", ""}, "", "falta el grupo"},
		{[]string{"2026003", "Eva", "x1", "", "IRD-71", ""}, "", "falta el tipo"},
		{[]string{"2026004", "Fer", "x1", "jefe", "", ""}, "", "tipo inválido"},
		{[]string{"Mal Matricula", "Gil", "x1", "ALUMNO", "IRD-71", ""}, "", "matrícula inválida"},
		{[]string{"2026005", "Hugo", "", "ALUMNO", "IRD-71", ""}, "", "falta la contraseña"},
	}
	for _, c := range casos {
		f, msg := validar(c.fila, idx, "general", 2)
		if c.contiene == "" {
			if msg != "" || f.Tipo != c.tipo {
				t.Errorf("%v: msg=%q tipo=%v", c.fila, msg, f)
			}
			if c.tipo != "alumnos" && f.Grupo != "" {
				t.Errorf("%v: solo los alumnos llevan grupo", c.fila)
			}
			continue
		}
		if !strings.Contains(msg, c.contiene) {
			t.Errorf("%v: msg=%q, se esperaba %q", c.fila, msg, c.contiene)
		}
	}
	if f, _ := validar([]string{"2026001", "Ana", "x1", "alumno", "ird-71", ""}, idx, "general", 2); f.Grupo != "IRD-71" {
		t.Errorf("el grupo se guarda en mayúsculas: %s", f.Grupo)
	}
}
