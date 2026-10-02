package repositorio

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrMatriculaOcupada = errors.New("ya existe un usuario con esa matrícula/clave")

// DatosUsuario: lo que el admin puede corregir a mano
type DatosUsuario struct {
	Matricula string // nueva matrícula/clave (puede ser igual a la actual)
	Nombre    string
	Grupo     *string // solo alumnos; nil = sin grupo
}

// tipoDe regresa el tipo del usuario ("" si no existe)
func tipoDe(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, matricula string) (string, error) {
	var tipo string
	err := q.QueryRowContext(ctx, `SELECT TIPO FROM USUARIOS WHERE MATRICULA = ?`, matricula).Scan(&tipo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(tipo), err
}

// ActualizarDatos cambia matrícula, nombre y grupo en una transacción.
// Si cambia la matrícula, Firebird la actualiza sola en roles, grupos y labs (ON UPDATE CASCADE).
func (r *UsuarioRepo) ActualizarDatos(ctx context.Context, actual string, d *DatosUsuario) error {
	if actual == r.AdminClave {
		return ErrProtegido
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tipo, err := tipoDe(ctx, tx, actual)
	if err != nil {
		return err
	}
	if tipo == "" {
		return ErrNoEncontrado
	}

	if d.Matricula != actual {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM USUARIOS WHERE MATRICULA = ?`, d.Matricula).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrMatriculaOcupada
		}
	}

	// Solo los alumnos tienen grupo; si el grupo no existe se crea
	if tipo != "alumnos" {
		d.Grupo = nil
	}
	if d.Grupo != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE OR INSERT INTO GRUPOS (CODIGO) VALUES (?) MATCHING (CODIGO)`, *d.Grupo); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE USUARIOS SET MATRICULA = ?, NOMBRE = ?, GRUPO = ?, ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE MATRICULA = ?`, d.Matricula, d.Nombre, d.Grupo, actual); err != nil {
		return err
	}
	return tx.Commit()
}

// Eliminar borra al usuario. Sus roles, asignaciones a grupos, laboratorios
// e invitaciones se borran solos (ON DELETE CASCADE).
func (r *UsuarioRepo) Eliminar(ctx context.Context, matricula string) error {
	if matricula == r.AdminClave {
		return ErrProtegido
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tipo, err := tipoDe(ctx, tx, matricula)
	if err != nil {
		return err
	}
	if tipo == "" {
		return ErrNoEncontrado
	}

	// Si es admin activo, debe quedar otro
	var esAdmin int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM USUARIOS U WHERE U.MATRICULA = ? AND U.ACTIVO = TRUE
		  AND EXISTS (SELECT 1 FROM USUARIO_ROLES R WHERE R.MATRICULA = U.MATRICULA AND R.ROL = 'ADMIN')`,
		matricula).Scan(&esAdmin); err != nil {
		return err
	}
	if esAdmin > 0 {
		var otros int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM USUARIOS U WHERE U.ACTIVO = TRUE AND U.MATRICULA <> ?
			  AND EXISTS (SELECT 1 FROM USUARIO_ROLES R WHERE R.MATRICULA = U.MATRICULA AND R.ROL = 'ADMIN')`,
			matricula).Scan(&otros); err != nil {
			return err
		}
		if otros == 0 {
			return ErrUltimoAdmin
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM USUARIOS WHERE MATRICULA = ?`, matricula); err != nil {
		return err
	}
	return tx.Commit()
}
