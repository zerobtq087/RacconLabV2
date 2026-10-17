package domain

// Importacion: avance de una carga de usuarios (igual que models/Importacion.ts)
type Importacion struct {
	ID           string      `gorm:"column:ID" json:"id"`
	Tipo         string      `gorm:"column:TIPO" json:"tipo"`
	Archivo      string      `gorm:"column:ARCHIVO" json:"archivo"`
	Estado       string      `gorm:"column:ESTADO" json:"estado"`
	Mensaje      string      `gorm:"column:MENSAJE" json:"mensaje"`
	Porcentaje   int         `gorm:"column:PORCENTAJE" json:"porcentaje"`
	Leidas       int         `gorm:"column:LEIDAS" json:"leidas"`
	Insertados   int         `gorm:"column:INSERTADOS" json:"insertados"`
	Actualizados int         `gorm:"column:ACTUALIZADOS" json:"actualizados"`
	ConError     int         `gorm:"column:CON_ERROR" json:"con_error"`
	Errores      []ErrorFila `gorm:"-" json:"errores"`
}

type ErrorFila struct {
	Fila    int    `gorm:"column:FILA" json:"fila"`
	Mensaje string `gorm:"column:MENSAJE" json:"mensaje"`
}

// FilaImport: un renglón del archivo ya validado, listo para guardar
type FilaImport struct {
	Num       int
	Matricula string
	Nombre    string
	Password  string // en claro solo mientras se cifra
	Hash      string
	Grupo     string // solo alumnos
	Carrera   string // solo alumnos (opcional)
}
