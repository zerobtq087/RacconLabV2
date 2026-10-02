// Package repositorio: acceso a Firebird. Cada repo recibe el *sql.DB compartido.
package repositorio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"raccoon_lab/internal/modelos"
)

type UsuarioRepo struct {
	DB *sql.DB
}

// BuscarPorMatricula devuelve (nil, nil) si no existe.
// Regresa un puntero: el usuario se crea una vez y se pasa sin copiarlo.
func (r *UsuarioRepo) BuscarPorMatricula(ctx context.Context, matricula string) (*modelos.Usuario, error) {
	matricula = strings.ToUpper(strings.TrimSpace(matricula))

	u := &modelos.Usuario{}
	var grupo sql.NullString
	err := r.DB.QueryRowContext(ctx, `
		SELECT MATRICULA, NOMBRE, TIPO, GRUPO, ACTIVO, PASSWORD_HASH
		FROM USUARIOS WHERE MATRICULA = ?`, matricula,
	).Scan(&u.Matricula, &u.Nombre, &u.Tipo, &grupo, &u.Activo, &u.PasswordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buscando usuario %s: %w", matricula, err)
	}

	limpiar(u)
	if grupo.Valid && strings.TrimSpace(grupo.String) != "" {
		g := strings.TrimSpace(grupo.String)
		u.Grupo = &g
	}

	if err := r.cargarRoles(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// cargarRoles llena u.Roles directamente sobre el objeto original (puntero)
func (r *UsuarioRepo) cargarRoles(ctx context.Context, u *modelos.Usuario) error {
	filas, err := r.DB.QueryContext(ctx, `
		SELECT ROL FROM USUARIO_ROLES WHERE MATRICULA = ?
		ORDER BY CASE ROL WHEN 'ADMIN' THEN 1 WHEN 'PROFESOR' THEN 2 ELSE 3 END`, u.Matricula)
	if err != nil {
		return fmt.Errorf("leyendo roles de %s: %w", u.Matricula, err)
	}
	defer filas.Close()

	u.Roles = u.Roles[:0]
	for filas.Next() {
		var rol string
		if err := filas.Scan(&rol); err != nil {
			return err
		}
		u.Roles = append(u.Roles, strings.TrimSpace(rol))
	}
	return filas.Err()
}

// RegistrarAcceso guarda la fecha del último login
func (r *UsuarioRepo) RegistrarAcceso(ctx context.Context, matricula string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE USUARIOS SET ULTIMO_ACCESO = CURRENT_TIMESTAMP WHERE MATRICULA = ?`, matricula)
	return err
}

// limpiar quita espacios de relleno que Firebird puede devolver en CHAR/VARCHAR
func limpiar(u *modelos.Usuario) {
	u.Matricula = strings.TrimSpace(u.Matricula)
	u.Nombre = strings.TrimSpace(u.Nombre)
	u.Tipo = strings.TrimSpace(u.Tipo)
	u.PasswordHash = strings.TrimSpace(u.PasswordHash)
}
