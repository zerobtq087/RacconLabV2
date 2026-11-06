package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const tamLote = 500 // filas por transacción

// CerrarSesiones lo da el middleware (así el servicio no toca Redis)
type CerrarSesiones func(ctx context.Context, userID string)

// Importador carga usuarios desde CSV/Excel en segundo plano.
// Se crea UNA vez y se comparte por apuntador.
type Importador struct {
	repository *repository.ImportacionRepository
	cerrar     CerrarSesiones
	protegida  string // matrícula del admin general: la importación nunca la toca
}

func NewImportador(r *repository.ImportacionRepository, cerrar CerrarSesiones, protegida string) *Importador {
	return &Importador{repository: r, cerrar: cerrar, protegida: strings.ToLower(protegida)}
}

// trabajo: el avance en memoria mientras corre (se guarda en Firebird cada segundo)
type trabajo struct {
	mu        sync.Mutex
	imp       domain.Importacion
	errores   []domain.ErrorFila // pendientes de guardar
	guardados int                // errores ya guardados
}

func (t *trabajo) errorFila(fila int, msg string) {
	t.mu.Lock()
	t.imp.ConError++
	t.errores = append(t.errores, domain.ErrorFila{Fila: fila, Mensaje: msg})
	t.mu.Unlock()
}

// guardar manda a Firebird los contadores y los errores pendientes
func (im *Importador) guardar(t *trabajo) {
	t.mu.Lock()
	copia := t.imp
	pendientes := t.errores
	t.errores = nil
	yaGuardados := t.guardados
	t.guardados += len(pendientes)
	t.mu.Unlock()

	im.repository.SaveProgress(&copia)
	im.repository.SaveErrors(copia.ID, pendientes, yaGuardados)
}

// Process lee el archivo, cifra contraseñas en paralelo y guarda por lotes.
// Se llama con "go": el handler responde de inmediato.
func (im *Importador) Process(id, tipo, archivo, ruta string) {
	defer os.Remove(ruta)

	t := &trabajo{imp: domain.Importacion{ID: id, Tipo: tipo, Archivo: archivo, Estado: "PROCESANDO"}}

	// Latido: cada segundo guarda el avance para que el frontend lo vea
	listo := make(chan struct{})
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				im.guardar(t)
			case <-listo:
				return
			}
		}
	}()

	err := im.procesar(t, ruta)
	close(listo)
	im.guardar(t)

	t.mu.Lock()
	if err != nil {
		t.imp.Estado, t.imp.Mensaje = "ERROR", err.Error()
	} else {
		t.imp.Estado, t.imp.Porcentaje = "TERMINADO", 100
	}
	final := t.imp
	t.mu.Unlock()

	im.repository.Finish(&final)
	log.Printf("[IMPORTAR] %s: %s. %d nuevos, %d actualizados, %d con error",
		archivo, final.Estado, final.Insertados, final.Actualizados, final.ConError)
}

func (im *Importador) procesar(t *trabajo, ruta string) error {
	l, err := Abrir(ruta)
	if err != nil {
		return err
	}
	defer l.Cerrar()

	// Roles de cada tipo (catálogo TIPO_ROLES). En "general" cada fila trae su tipo.
	roles := map[string][]string{}
	for _, tipo := range []string{"alumnos", "maestros", "admins"} {
		if t.imp.Tipo == "general" || t.imp.Tipo == tipo {
			roles[tipo] = im.repository.RolesDeTipo(tipo)
		}
	}
	if len(roles) == 0 {
		return fmt.Errorf("tipo de importación desconocido: %s", t.imp.Tipo)
	}

	// Si la BD falla, se cancela todo para no seguir leyendo/cifrando en vano
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	validas := make(chan *domain.FilaImport, 1000)
	cifradas := make(chan *domain.FilaImport, 1000)

	// 1) Lector: valida filas y las manda a cifrar
	errLectura := make(chan error, 1)
	go func() {
		defer close(validas)
		errLectura <- im.leer(ctx, t, l, validas)
	}()

	// 2) Cifrado bcrypt en paralelo (uno por núcleo del CPU)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range validas {
				if ctx.Err() != nil {
					continue
				}
				h, err := bcrypt.GenerateFromPassword([]byte(f.Password), bcrypt.DefaultCost)
				if err != nil {
					t.errorFila(f.Num, "no se pudo cifrar la contraseña")
					continue
				}
				f.Hash, f.Password = string(h), "" // la contraseña en claro ya no se guarda
				cifradas <- f
			}
		}()
	}
	go func() { wg.Wait(); close(cifradas) }()

	// 3) Escritor: guarda en Firebird por lotes de 500
	if err := im.escribir(t, roles, cifradas); err != nil {
		cancelar()
		for range cifradas { // vaciar para que no se bloqueen los demás
		}
		return fmt.Errorf("error de base de datos: %w", err)
	}

	return <-errLectura
}

func (im *Importador) escribir(t *trabajo, roles map[string][]string, entrada <-chan *domain.FilaImport) error {
	grupos := make(map[string]string) // caché código -> ID, compartido entre lotes
	lote := make([]*domain.FilaImport, 0, tamLote)

	guardarLote := func() error {
		if len(lote) == 0 {
			return nil
		}
		res, err := im.repository.SaveBatch(roles, lote, grupos)
		if err != nil {
			return err
		}

		t.mu.Lock()
		t.imp.Insertados += res.Nuevos
		t.imp.Actualizados += len(res.Actualizados)
		t.mu.Unlock()
		for _, e := range res.Errores {
			t.errorFila(e.Fila, e.Mensaje)
		}

		// Si cambió la contraseña de alguien que ya existía, se cierran sus sesiones
		if im.cerrar != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			for _, id := range res.Actualizados {
				im.cerrar(ctx, id)
			}
			cancel()
		}

		lote = lote[:0]
		return nil
	}

	for f := range entrada {
		lote = append(lote, f)
		if len(lote) == tamLote {
			if err := guardarLote(); err != nil {
				return err
			}
		}
	}
	return guardarLote()
}

// ---------------------------------------------------------------------
// Lectura y validación de filas
// ---------------------------------------------------------------------

// Posición de cada campo en la fila
const (
	cID = iota
	cNombre
	cPassword
	cGrupo
	cCarrera
	cTipo
)

// sinonimos: cualquier encabezado que signifique lo mismo (ya normalizado:
// sin acentos, minúsculas y solo letras, así "Contraseña" -> "contrasena")
var sinonimos = map[string]int{
	"matricula": cID, "clave": cID, "matriculaclave": cID, "usuario": cID, "id": cID,
	"nombre": cNombre, "nombrecompleto": cNombre,
	"contrasenia": cPassword, "contrasena": cPassword, "password": cPassword,
	"grupo":   cGrupo,
	"carrera": cCarrera,
	"tipo":    cTipo, "tipousuario": cTipo, "rol": cTipo,
}

// Columnas de cada tipo de archivo, en el orden de la plantilla descargable.
// Si el archivo no trae encabezados, se lee en este orden.
var columnasImport = map[string][]string{
	"alumnos":  {"matricula", "nombre", "contraseña", "grupo", "carrera"},
	"maestros": {"matricula", "nombre", "contraseña"},
	"admins":   {"matricula", "nombre", "contraseña"},
	"general":  {"matricula", "nombre", "contraseña", "tipo", "grupo", "carrera"},
}

func (im *Importador) leer(ctx context.Context, t *trabajo, l Lector, salida chan<- *domain.FilaImport) error {
	var (
		indices []int
		num     int
		vistos  = make(map[string]int) // matrícula -> fila (para detectar repetidas)
	)
	tipoArchivo := t.imp.Tipo

	for {
		reg, err := l.Siguiente()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("fila %d: %w", num+1, err)
		}
		num++

		t.mu.Lock()
		t.imp.Leidas = num
		t.imp.Porcentaje = int(l.Progreso() * 99)
		t.mu.Unlock()

		if vacia(reg) {
			continue
		}

		// Primera fila con datos: ¿son encabezados?
		if indices == nil {
			if idx, ok := detectarEncabezados(reg, tipoArchivo); ok {
				indices = idx
				continue
			}
			// Sin encabezados: las columnas van en el orden de la plantilla
			indices = ordenPlantilla(tipoArchivo)
		}

		f, msg := validar(reg, indices, tipoArchivo, num)
		if msg != "" {
			t.errorFila(num, msg)
			continue
		}
		if f.Matricula == im.protegida {
			t.errorFila(num, "es el administrador general: no se modifica por importación")
			continue
		}
		if previa, ok := vistos[f.Matricula]; ok {
			t.errorFila(num, fmt.Sprintf("matrícula %s repetida (ya venía en la fila %d)", f.Matricula, previa))
			continue
		}
		vistos[f.Matricula] = num

		select {
		case salida <- f:
		case <-ctx.Done():
			return nil
		}
	}
}

func vacia(reg []string) bool {
	for _, c := range reg {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// normalizar: "Contraseña" -> "contrasena", "Matrícula/Clave" -> "matriculaclave"
func normalizar(s string) string {
	tr := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	s, _, _ = transform.String(tr, strings.ToLower(strings.TrimSpace(s)))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || r == '_' {
			return r
		}
		return -1
	}, s)
}

func detectarEncabezados(reg []string, tipoArchivo string) ([]int, bool) {
	idx := []int{-1, -1, -1, -1, -1, -1}
	for i, c := range reg {
		if campo, ok := sinonimos[normalizar(c)]; ok && idx[campo] == -1 {
			idx[campo] = i
		}
	}
	if idx[cID] == -1 || idx[cNombre] == -1 || idx[cPassword] == -1 {
		return nil, false
	}
	if tipoArchivo == "alumnos" && idx[cGrupo] == -1 {
		return nil, false
	}
	if tipoArchivo == "general" && idx[cTipo] == -1 {
		return nil, false
	}
	return idx, true
}

// ordenPlantilla: posición de cada campo cuando el archivo no trae encabezados
func ordenPlantilla(tipoArchivo string) []int {
	idx := []int{-1, -1, -1, -1, -1, -1}
	for i, col := range columnasImport[tipoArchivo] {
		idx[sinonimos[normalizar(col)]] = i
	}
	return idx
}

func celda(reg []string, i int) string {
	if i < 0 || i >= len(reg) {
		return ""
	}
	return strings.TrimSpace(reg[i])
}

func validar(reg []string, idx []int, tipoArchivo string, num int) (*domain.FilaImport, string) {
	f := &domain.FilaImport{
		Num:       num,
		Tipo:      tipoArchivo,
		Matricula: celda(reg, idx[cID]),
		Nombre:    celda(reg, idx[cNombre]),
		Password:  celda(reg, idx[cPassword]),
		Grupo:     celda(reg, idx[cGrupo]),
		Carrera:   celda(reg, idx[cCarrera]),
	}

	// En el archivo general cada fila dice su tipo
	if tipoArchivo == "general" {
		texto := celda(reg, idx[cTipo])
		tipo, ok := tipoDeTexto(texto)
		switch {
		case texto == "":
			return nil, "falta el tipo (ALUMNO, PROFESOR o ADMIN)"
		case !ok:
			return nil, fmt.Sprintf("tipo inválido %q (usa ALUMNO, PROFESOR o ADMIN)", texto)
		}
		f.Tipo = tipo
	}

	// Las mismas reglas que el alta manual (service/usuarios.go)
	if msg := validarDatos(&f.Matricula, &f.Nombre, &f.Grupo, &f.Carrera, f.Tipo == "alumnos"); msg != "" {
		return nil, msg
	}
	switch {
	case f.Password == "":
		return nil, "falta la contraseña"
	case len(f.Password) > 72:
		return nil, "la contraseña pasa de 72 caracteres"
	}
	return f, ""
}
