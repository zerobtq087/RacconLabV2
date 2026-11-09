package domain

import "time"

// ---------------------------------------------------------------------
// Tablero de Inicio (igual que models/Inicio.ts). GET /api/inicio llena
// SOLO la parte del rol con el que se entró; las otras van vacías (nil).
// ---------------------------------------------------------------------

type Inicio struct {
	Rol      string          `json:"rol"`
	Alumno   *InicioAlumno   `json:"alumno,omitempty"`
	Profesor *InicioProfesor `json:"profesor,omitempty"`
	Admin    *InicioAdmin    `json:"admin,omitempty"`
}

// InicioAlumno: su lab, lo que le falta entregar, sus calificaciones e invitaciones
type InicioAlumno struct {
	Lab            *Laboratorio `json:"lab"`
	SoyDueno       bool         `json:"soy_dueno"`
	Pendientes     []MiPractica `json:"pendientes"`     // activas sin entregar
	Calificaciones []MiPractica `json:"calificaciones"` // las últimas entregadas
	Invitaciones   []Invitacion `json:"invitaciones"`
}

// InicioProfesor: sus prácticas activas y sus grupos
type InicioProfesor struct {
	Practicas      []ResumenPractica `json:"practicas"`
	Grupos         []ResumenGrupo    `json:"grupos"`
	Alumnos        int               `json:"alumnos"`         // en todos sus grupos
	LabsEncendidos int               `json:"labs_encendidos"` // de sus alumnos
}

type ResumenPractica struct {
	ID         string `json:"id"`
	Nombre     string `json:"nombre"`
	Grupo      string `json:"grupo"`
	Entregadas int    `json:"entregadas"`
	Alumnos    int    `json:"alumnos"`
}

type ResumenGrupo struct {
	ID             string `gorm:"column:ID" json:"id"`
	Codigo         string `gorm:"column:CODIGO" json:"codigo"`
	Carrera        string `gorm:"column:CARRERA" json:"carrera"`
	Alumnos        int    `gorm:"column:ALUMNOS" json:"alumnos"`
	LabsEncendidos int    `gorm:"column:LABS" json:"labs_encendidos"`
}

// InicioAdmin: estado del servidor y de la plataforma
type InicioAdmin struct {
	LabsActivos       int                 `json:"labs_activos"`
	LabsCreando       int                 `json:"labs_creando"`
	LabsMax           int                 `json:"labs_max"`
	Servidor          RecursosServidor    `json:"servidor"`
	Usuarios          []UsuariosPorTipo   `json:"usuarios"`
	UltimaImportacion *ResumenImportacion `json:"ultima_importacion"`
	PracticasActivas  int                 `json:"practicas_activas"`
	Servicios         []EstadoServicio    `json:"servicios"`
}

// RecursosServidor: CPU, RAM y disco del servidor (bytes; CPU en %)
type RecursosServidor struct {
	CPU        float64 `json:"cpu"`
	Nucleos    int     `json:"nucleos"`
	RAMTotal   uint64  `json:"ram_total"`
	RAMUsada   uint64  `json:"ram_usada"`
	DiscoTotal uint64  `json:"disco_total"`
	DiscoUsado uint64  `json:"disco_usado"`
	DiscoRuta  string  `json:"disco_ruta"` // dónde viven los labs (GNS3_VOLUMES)
	Error      string  `json:"error"`
}

type UsuariosPorTipo struct {
	Tipo        string `gorm:"column:TIPO" json:"tipo"`
	Descripcion string `gorm:"column:DESCRIPCION" json:"descripcion"`
	Total       int    `gorm:"column:TOTAL" json:"total"`
	Activos     int    `gorm:"column:ACTIVOS" json:"activos"`
}

type ResumenImportacion struct {
	Archivo      string    `gorm:"column:ARCHIVO" json:"archivo"`
	Tipo         string    `gorm:"column:TIPO" json:"tipo"`
	Estado       string    `gorm:"column:ESTADO" json:"estado"`
	Insertados   int       `gorm:"column:INSERTADOS" json:"insertados"`
	Actualizados int       `gorm:"column:ACTUALIZADOS" json:"actualizados"`
	ConError     int       `gorm:"column:CON_ERROR" json:"con_error"`
	Inicio       time.Time `gorm:"column:INICIO" json:"inicio"`
	Usuario      string    `gorm:"column:USUARIO" json:"usuario"`
}

type EstadoServicio struct {
	Nombre  string `json:"nombre"`
	Bien    bool   `json:"bien"`
	Detalle string `json:"detalle"`
}
