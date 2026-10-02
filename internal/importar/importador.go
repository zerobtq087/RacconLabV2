package importar

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ---------------------------------------------------------------------
// Tipos de archivo
// ---------------------------------------------------------------------

// Tipo describe cada uno de los 3 archivos: columnas y roles que otorga
type Tipo struct {
	Nombre   string
	Columnas []string // en orden: así deben venir si el archivo no trae encabezados
	Roles    []string
}

var Tipos = map[string]*Tipo{
	"alumnos":  {Nombre: "alumnos", Columnas: []string{"matricula", "nombre", "contrasenia", "grupo"}, Roles: []string{"ALUMNO"}},
	"maestros": {Nombre: "maestros", Columnas: []string{"clave", "nombre", "contrasenia"}, Roles: []string{"PROFESOR", "ALUMNO"}},
	"admins":   {Nombre: "admins", Columnas: []string{"clave", "nombre", "contrasenia"}, Roles: []string{"ADMIN", "PROFESOR", "ALUMNO"}},
}

// sinonimos: cualquier encabezado que signifique lo mismo
var sinonimos = map[string]string{
	"matricula": "id", "clave": "id", "matriculaclave": "id", "usuario": "id", "id": "id",
	"nombre": "nombre", "nombrecompleto": "nombre",
	"contrasenia": "password", "contrasena": "password", "password": "password", "clave_acceso": "password",
	"grupo": "grupo",
}

// ---------------------------------------------------------------------
// Trabajo: una importación en curso (vive en memoria de la app)
// ---------------------------------------------------------------------

type ErrorFila struct {
	Fila    int    `json:"fila"`
	Mensaje string `json:"mensaje"`
}

const maxErroresGuardados = 200

type Trabajo struct {
	mu sync.Mutex

	ID           string      `json:"id"`
	Tipo         string      `json:"tipo"`
	Archivo      string      `json:"archivo"`
	Estado       string      `json:"estado"` // procesando | terminado | error
	Mensaje      string      `json:"mensaje,omitempty"`
	Porcentaje   int         `json:"porcentaje"`
	Leidas       int         `json:"leidas"`
	Insertados   int         `json:"insertados"`
	Actualizados int         `json:"actualizados"`
	ConError     int         `json:"con_error"`
	Errores      []ErrorFila `json:"errores"`
	Inicio       time.Time   `json:"inicio"`
	Fin          *time.Time  `json:"fin,omitempty"`

	ruta string
}

func (t *Trabajo) errorFila(fila int, msg string) {
	t.mu.Lock()
	t.ConError++
	if len(t.Errores) < maxErroresGuardados {
		t.Errores = append(t.Errores, ErrorFila{Fila: fila, Mensaje: msg})
	}
	t.mu.Unlock()
}

// Copia devuelve una foto del estado (para responder JSON sin carreras)
func (t *Trabajo) Copia() *Trabajo {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := &Trabajo{
		ID: t.ID, Tipo: t.Tipo, Archivo: t.Archivo, Estado: t.Estado, Mensaje: t.Mensaje,
		Porcentaje: t.Porcentaje, Leidas: t.Leidas, Insertados: t.Insertados,
		Actualizados: t.Actualizados, ConError: t.ConError, Inicio: t.Inicio, Fin: t.Fin,
	}
	c.Errores = slices.Clone(t.Errores)
	return c
}

// ---------------------------------------------------------------------
// Importador: se crea una vez y se comparte por puntero
// ---------------------------------------------------------------------

// CerrarSesiones lo implementa el store de Redis (se inyecta para no acoplar paquetes)
type CerrarSesiones func(ctx context.Context, matricula string) error

type Importador struct {
	DB             *sql.DB
	CerrarSesiones CerrarSesiones
	Dir            string // carpeta temporal donde se guarda el archivo subido
	Protegida      string // clave del admin general: la importación nunca la toca

	mu       sync.Mutex
	trabajos map[string]*Trabajo
	activo   *Trabajo // solo una importación a la vez (bcrypt usa todo el CPU)
}

var ErrOcupado = errors.New("ya hay una importación en curso, espera a que termine")

func Nuevo(db *sql.DB, cerrar CerrarSesiones, dir string) *Importador {
	return &Importador{DB: db, CerrarSesiones: cerrar, Dir: dir, trabajos: make(map[string]*Trabajo)}
}

func nuevoID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Reservar crea el trabajo y la ruta temporal donde se debe guardar el archivo
func (im *Importador) Reservar(tipo, archivo string) (*Trabajo, error) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.activo != nil {
		return nil, ErrOcupado
	}
	ext := strings.ToLower(archivo[strings.LastIndex(archivo, ".")+1:])
	t := &Trabajo{
		ID:      nuevoID(),
		Tipo:    tipo,
		Archivo: archivo,
		Estado:  "procesando",
		Inicio:  time.Now(),
		Errores: []ErrorFila{},
	}
	t.ruta = fmt.Sprintf("%s/raccoon-import-%s.%s", im.Dir, t.ID, ext)
	im.activo = t
	im.trabajos[t.ID] = t
	im.limpiarViejos()
	return t, nil
}

// Cancelar libera la reserva si la subida falló
func (im *Importador) Cancelar(t *Trabajo, motivo string) {
	os.Remove(t.ruta)
	im.terminar(t, "error", motivo)
}

func (im *Importador) Ruta(t *Trabajo) string { return t.ruta }

func (im *Importador) Obtener(id string) *Trabajo {
	im.mu.Lock()
	defer im.mu.Unlock()
	if t, ok := im.trabajos[id]; ok {
		return t.Copia()
	}
	return nil
}

// limpiarViejos conserva solo los últimos 20 trabajos
func (im *Importador) limpiarViejos() {
	if len(im.trabajos) <= 20 {
		return
	}
	var masViejo *Trabajo
	for _, t := range im.trabajos {
		if t != im.activo && (masViejo == nil || t.Inicio.Before(masViejo.Inicio)) {
			masViejo = t
		}
	}
	if masViejo != nil {
		delete(im.trabajos, masViejo.ID)
	}
}

func (im *Importador) terminar(t *Trabajo, estado, mensaje string) {
	ahora := time.Now()
	t.mu.Lock()
	t.Estado, t.Mensaje, t.Fin = estado, mensaje, &ahora
	if estado == "terminado" {
		t.Porcentaje = 100
	}
	t.mu.Unlock()

	im.mu.Lock()
	if im.activo == t {
		im.activo = nil
	}
	im.mu.Unlock()
}

// ---------------------------------------------------------------------
// Procesamiento (se ejecuta en segundo plano)
// ---------------------------------------------------------------------

// fila validada lista para guardar. Viaja por los canales como PUNTERO.
type fila struct {
	num      int
	id       string
	nombre   string
	password string
	grupo    *string
	hash     string
}

const tamLote = 500

// Procesar lee el archivo, cifra contraseñas en paralelo y guarda por lotes
func (im *Importador) Procesar(t *Trabajo) {
	defer os.Remove(t.ruta)
	tipo := Tipos[t.Tipo]

	l, err := Abrir(t.ruta)
	if err != nil {
		im.terminar(t, "error", err.Error())
		return
	}
	defer l.Cerrar()

	log.Printf("[IMPORTAR] %s: inicia %s (%s)", t.ID, t.Archivo, t.Tipo)

	// Si la BD falla, se cancela todo para no seguir leyendo/cifrando en vano
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	validas := make(chan *fila, 1000)
	cifradas := make(chan *fila, 1000)

	// 1) Lector: valida filas y las manda a cifrar
	errLectura := make(chan error, 1)
	go func() {
		defer close(validas)
		errLectura <- im.leer(ctx, t, tipo, l, validas)
	}()

	// 2) Cifrado bcrypt en paralelo (uno por núcleo)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range validas {
				if ctx.Err() != nil {
					continue
				}
				h, err := bcrypt.GenerateFromPassword([]byte(f.password), bcrypt.DefaultCost)
				if err != nil {
					t.errorFila(f.num, "no se pudo cifrar la contraseña")
					continue
				}
				f.hash, f.password = string(h), "" // la contraseña en claro ya no se guarda
				cifradas <- f
			}
		}()
	}
	go func() { wg.Wait(); close(cifradas) }()

	// 3) Escritor: guarda en Firebird por lotes de 500
	if err := im.escribir(t, tipo, cifradas); err != nil {
		cancelar()
		for range cifradas { // vaciar para que no se bloqueen los demás
		}
		im.terminar(t, "error", "error de base de datos: "+err.Error())
		return
	}

	if err := <-errLectura; err != nil {
		im.terminar(t, "error", err.Error())
		return
	}

	c := t.Copia()
	log.Printf("[IMPORTAR] %s: listo. %d nuevos, %d actualizados, %d con error",
		t.ID, c.Insertados, c.Actualizados, c.ConError)
	im.terminar(t, "terminado", "")
}

var reID = regexp.MustCompile(`^[a-z0-9._-]{1,20}$`)

func (im *Importador) leer(ctx context.Context, t *Trabajo, tipo *Tipo, l Lector, salida chan<- *fila) error {
	var (
		indices []int // posición de: id, nombre, password, grupo
		num     int
		vistos  = make(map[string]int) // matrícula -> fila (para detectar duplicados)
	)

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
		t.Leidas = num
		t.Porcentaje = int(l.Progreso() * 99)
		t.mu.Unlock()

		if vacia(reg) {
			continue
		}

		// Primera fila con datos: ¿son encabezados?
		if indices == nil {
			if idx, ok := detectarEncabezados(reg, tipo); ok {
				indices = idx
				continue
			}
			indices = posicionales(tipo)
		}

		f, msg := validar(reg, indices, tipo, num)
		if msg != "" {
			t.errorFila(num, msg)
			continue
		}
		if f.id == im.Protegida {
			t.errorFila(num, "es el administrador general: no se modifica por importación")
			continue
		}
		if previa, ok := vistos[f.id]; ok {
			t.errorFila(num, fmt.Sprintf("matrícula %s repetida (ya venía en la fila %d)", f.id, previa))
			continue
		}
		vistos[f.id] = num
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

func detectarEncabezados(reg []string, tipo *Tipo) ([]int, bool) {
	idx := []int{-1, -1, -1, -1}
	pos := map[string]int{"id": 0, "nombre": 1, "password": 2, "grupo": 3}
	for i, c := range reg {
		if campo, ok := sinonimos[normalizar(c)]; ok && idx[pos[campo]] == -1 {
			idx[pos[campo]] = i
		}
	}
	if idx[0] == -1 || idx[1] == -1 || idx[2] == -1 {
		return nil, false
	}
	if tipo.Nombre == "alumnos" && idx[3] == -1 {
		return nil, false
	}
	return idx, true
}

func posicionales(tipo *Tipo) []int {
	if tipo.Nombre == "alumnos" {
		return []int{0, 1, 2, 3}
	}
	return []int{0, 1, 2, -1}
}

func celda(reg []string, i int) string {
	if i < 0 || i >= len(reg) {
		return ""
	}
	return strings.TrimSpace(reg[i])
}

func validar(reg []string, idx []int, tipo *Tipo, num int) (*fila, string) {
	f := &fila{
		num:      num,
		id:       strings.ToLower(celda(reg, idx[0])),
		nombre:   strings.Join(strings.Fields(celda(reg, idx[1])), " "),
		password: celda(reg, idx[2]),
	}
	switch {
	case f.id == "":
		return nil, "falta la matrícula/clave"
	case !reID.MatchString(f.id):
		return nil, fmt.Sprintf("matrícula/clave inválida %q (solo letras, números, . _ -; máx. 20)", f.id)
	case f.nombre == "":
		return nil, "falta el nombre"
	case utf8.RuneCountInString(f.nombre) > 120:
		return nil, "el nombre pasa de 120 caracteres"
	case f.password == "":
		return nil, "falta la contraseña"
	case len(f.password) > 72:
		return nil, "la contraseña pasa de 72 caracteres"
	}
	if tipo.Nombre == "alumnos" {
		g := strings.ToUpper(celda(reg, idx[3]))
		switch {
		case g == "":
			return nil, "falta el grupo"
		case utf8.RuneCountInString(g) > 15:
			return nil, "el grupo pasa de 15 caracteres"
		}
		f.grupo = &g
	}
	return f, ""
}

func (im *Importador) escribir(t *Trabajo, tipo *Tipo, entrada <-chan *fila) error {
	gruposListos := make(map[string]bool)
	lote := make([]*fila, 0, tamLote)

	guardar := func() error {
		if len(lote) == 0 {
			return nil
		}
		actualizados, err := im.guardarLote(t, tipo, lote, gruposListos)
		if err != nil {
			return err
		}
		// Si se cambió la contraseña de alguien, se cierran sus sesiones
		if im.CerrarSesiones != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			for _, m := range actualizados {
				_ = im.CerrarSesiones(ctx, m)
			}
			cancel()
		}
		lote = lote[:0]
		return nil
	}

	for f := range entrada {
		lote = append(lote, f)
		if len(lote) == tamLote {
			if err := guardar(); err != nil {
				return err
			}
		}
	}
	return guardar()
}

// guardarLote: una transacción por lote. Un error en una fila no tumba el lote.
func (im *Importador) guardarLote(t *Trabajo, tipo *Tipo, lote []*fila, gruposListos map[string]bool) ([]string, error) {
	ctx := context.Background()
	tx, err := im.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var nuevos, actualizados int
	var matriculasActualizadas []string

	for _, f := range lote {
		if f.grupo != nil && !gruposListos[*f.grupo] {
			if _, err := tx.ExecContext(ctx,
				`UPDATE OR INSERT INTO GRUPOS (CODIGO) VALUES (?) MATCHING (CODIGO)`, *f.grupo); err != nil {
				t.errorFila(f.num, "no se pudo crear el grupo "+*f.grupo+": "+err.Error())
				continue
			}
			gruposListos[*f.grupo] = true
		}

		res, err := tx.ExecContext(ctx, `
			UPDATE USUARIOS SET NOMBRE = ?, PASSWORD_HASH = ?, TIPO = ?, GRUPO = ?,
			       ACTUALIZADO = CURRENT_TIMESTAMP
			WHERE MATRICULA = ?`,
			f.nombre, f.hash, tipo.Nombre, f.grupo, f.id)
		if err != nil {
			t.errorFila(f.num, "error al actualizar: "+err.Error())
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO USUARIOS (MATRICULA, NOMBRE, PASSWORD_HASH, TIPO, GRUPO)
				VALUES (?, ?, ?, ?, ?)`,
				f.id, f.nombre, f.hash, tipo.Nombre, f.grupo); err != nil {
				t.errorFila(f.num, "error al insertar: "+err.Error())
				continue
			}
			nuevos++
		} else {
			actualizados++
			matriculasActualizadas = append(matriculasActualizadas, f.id)
		}

		// Roles: se AGREGAN; no se quitan los que el admin ya haya asignado
		for _, rol := range tipo.Roles {
			if _, err := tx.ExecContext(ctx,
				`UPDATE OR INSERT INTO USUARIO_ROLES (MATRICULA, ROL) VALUES (?, ?) MATCHING (MATRICULA, ROL)`,
				f.id, rol); err != nil {
				t.errorFila(f.num, "error al asignar rol "+rol+": "+err.Error())
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.Insertados += nuevos
	t.Actualizados += actualizados
	t.mu.Unlock()
	return matriculasActualizadas, nil
}
