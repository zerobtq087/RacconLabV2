package domain

import "time"

// Usuario: una fila de USUARIOS con su grupo y sus roles
type Usuario struct {
	ID           string     `gorm:"column:ID" json:"id"`
	Matricula    string     `gorm:"column:MATRICULA" json:"matricula"`
	Nombre       string     `gorm:"column:NOMBRE" json:"nombre"`
	Password     string     `gorm:"column:PASSWORD" json:"-"`
	Tipo         string     `gorm:"column:TIPO" json:"tipo"`
	Grupo        string     `gorm:"column:GRUPO" json:"grupo"`
	Activo       bool       `gorm:"column:ACTIVO" json:"activo"`
	UltimoAcceso *time.Time `gorm:"column:ULTIMO_ACCESO" json:"ultimo_acceso"`
	Roles        []string   `gorm:"-" json:"roles"`
}

// UserAccount: lo que recibe el frontend en /api/current-user-acc
// (igual que models/UserAccount.ts)
type UserAccount struct {
	ID        string   `json:"id"`
	Matricula string   `json:"matricula"`
	Nombre    string   `json:"nombre"`
	Rol       string   `json:"rol"`   // rol con el que entró
	Roles     []string `json:"roles"` // todos sus roles
	Grupo     string   `json:"grupo"`
}
