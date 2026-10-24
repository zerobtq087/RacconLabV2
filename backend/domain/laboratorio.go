package domain

import "time"

// Laboratorio: un laboratorio GNS3 (un contenedor Docker)
type Laboratorio struct {
	ID         string    `gorm:"column:ID" json:"-"`
	IDDueno    string    `gorm:"column:ID_DUENO" json:"-"`
	Contenedor string    `gorm:"column:CONTENEDOR" json:"-"`
	PuertoBase int       `gorm:"column:PUERTO_BASE" json:"puerto"`
	Token      string    `gorm:"column:TOKEN" json:"token"`
	Codigo     string    `gorm:"column:CODIGO_EQUIPO" json:"codigo"`
	Estado     string    `gorm:"column:ESTADO" json:"estado"`
	Mensaje    string    `gorm:"column:MENSAJE" json:"mensaje"`
	Inicio     time.Time `gorm:"column:INICIO" json:"inicio"`
	Dueno      string    `gorm:"column:DUENO" json:"dueno"`     // nombre del dueño
	Usuario    string    `gorm:"column:USUARIO" json:"usuario"` // usuario de GNS3 (matrícula del dueño)
}

// EstadoLab: lo que ve el alumno en "Mi laboratorio" (igual que models/Laboratorio.ts)
type EstadoLab struct {
	*Laboratorio
	SoyDueno bool      `json:"soy_dueno"`
	Miembros []Persona `json:"miembros"`
}

// LabActivo: una fila del monitor del admin (igual que models/LabActivo.ts)
type LabActivo struct {
	ID        string    `gorm:"column:ID" json:"id"`
	Matricula string    `gorm:"column:MATRICULA" json:"matricula"`
	Nombre    string    `gorm:"column:NOMBRE" json:"nombre"`
	Grupo     string    `gorm:"column:GRUPO" json:"grupo"`
	Puerto    int       `gorm:"column:PUERTO_BASE" json:"puerto"`
	Codigo    string    `gorm:"column:CODIGO_EQUIPO" json:"codigo"`
	Estado    string    `gorm:"column:ESTADO" json:"estado"`
	Miembros  int       `gorm:"column:MIEMBROS" json:"miembros"`
	Inicio    time.Time `gorm:"column:INICIO" json:"inicio"`
}

// Invitacion: invitación pendiente que recibió el alumno
type Invitacion struct {
	ID        string    `gorm:"column:ID" json:"id"`
	Emisor    string    `gorm:"column:EMISOR" json:"emisor"`
	Matricula string    `gorm:"column:MATRICULA" json:"matricula"`
	Creada    time.Time `gorm:"column:CREADA" json:"creada"`
}
