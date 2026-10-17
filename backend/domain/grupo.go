package domain

// Persona: datos mínimos de un usuario (para listas)
type Persona struct {
	ID        string `gorm:"column:ID" json:"id"`
	Matricula string `gorm:"column:MATRICULA" json:"matricula"`
	Nombre    string `gorm:"column:NOMBRE" json:"nombre"`
}

type Grupo struct {
	ID       string    `gorm:"column:ID" json:"id"`
	Codigo   string    `gorm:"column:CODIGO" json:"codigo"`
	Carrera  string    `gorm:"column:CARRERA" json:"carrera"`
	Alumnos  int       `gorm:"column:ALUMNOS" json:"alumnos"`
	Docentes []Persona `gorm:"-" json:"docentes"`
}

type Carrera struct {
	ID     string `gorm:"column:ID" json:"id"`
	Nombre string `gorm:"column:NOMBRE" json:"nombre"`
}
