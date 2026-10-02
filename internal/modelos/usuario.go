// Package modelos: estructuras de datos compartidas.
package modelos

import "slices"

// Usuario tal como vive en Firebird.
// Grupo es *string: nil = sin grupo (maestros y admins), distinto de "".
type Usuario struct {
	Matricula    string   `json:"matricula"`
	Nombre       string   `json:"nombre"`
	Tipo         string   `json:"tipo"`
	Grupo        *string  `json:"grupo"`
	Activo       bool     `json:"activo"`
	Roles        []string `json:"roles"`
	PasswordHash string   `json:"-"` // nunca sale en el JSON
}

// TieneRol usa receptor puntero: no copia el usuario (ni su slice de roles)
func (u *Usuario) TieneRol(rol string) bool {
	return slices.Contains(u.Roles, rol)
}

// Sesion es lo que se guarda en Redis.
type Sesion struct {
	ID        string   `json:"-"`
	Matricula string   `json:"matricula"`
	Nombre    string   `json:"nombre"`
	Tipo      string   `json:"tipo"`
	Grupo     *string  `json:"grupo"`
	Roles     []string `json:"roles"`
	RolActivo string   `json:"rol_activo"`
	CreadaEn  int64    `json:"creada_en"`
}

func (s *Sesion) TieneRol(rol string) bool {
	return slices.Contains(s.Roles, rol)
}
