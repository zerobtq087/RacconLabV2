package repositorio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"raccoon_lab/internal/modelos"
)

// ---------------------------------------------------------------------
// Listado paginado (la paginación la hace Firebird, no el navegador)
// ---------------------------------------------------------------------

// FiltroUsuarios llega desde la URL: ?pagina=1&por_pagina=10&q=...&tipo=...&rol=...&grupo=...
type FiltroUsuarios struct {
	Pagina    int
	PorPagina int
	Busqueda  string
	Tipo      string // alumnos | maestros | admins | ""
	Rol       string // PROFESOR | ADMIN | ""
	Grupo     string
	Orden     string // matricula | nombre | tipo | grupo | activo
	Desc      bool
}

type PaginaUsuarios struct {
	Items []*modelos.Usuario `json:"items"`
	Total int                `json:"total"`
}

// Solo estas columnas se pueden usar para ordenar (evita inyección SQL)
var columnasOrden = map[string]string{
	"matricula": "U.MATRICULA",
	"nombre":    "U.NOMBRE",
	"tipo":      "U.TIPO",
	"grupo":     "U.GRUPO",
	"activo":    "U.ACTIVO",
}

func (f *FiltroUsuarios) where() (string, []any) {
	var cond []string
	var args []any
	if q := strings.TrimSpace(f.Busqueda); q != "" {
		// CONTAINING: búsqueda sin importar mayúsculas
		cond = append(cond, "(U.NOMBRE CONTAINING ? OR U.MATRICULA CONTAINING ?)")
		args = append(args, q, q)
	}
	if f.Tipo != "" {
		cond = append(cond, "U.TIPO = ?")
		args = append(args, f.Tipo)
	}
	if f.Grupo != "" {
		cond = append(cond, "U.GRUPO = ?")
		args = append(args, f.Grupo)
	}
	if f.Rol != "" {
		cond = append(cond, "EXISTS (SELECT 1 FROM USUARIO_ROLES R WHERE R.MATRICULA = U.MATRICULA AND R.ROL = ?)")
		args = append(args, f.Rol)
	}
	if len(cond) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(cond, " AND "), args
}

func (r *UsuarioRepo) Listar(ctx context.Context, f *FiltroUsuarios) (*PaginaUsuarios, error) {
	where, args := f.where()

	res := &PaginaUsuarios{Items: []*modelos.Usuario{}}
	if err := r.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM USUARIOS U"+where, args...).Scan(&res.Total); err != nil {
		return nil, fmt.Errorf("contando usuarios: %w", err)
	}
	if res.Total == 0 {
		return res, nil
	}

	col, ok := columnasOrden[f.Orden]
	if !ok {
		col = "U.MATRICULA"
	}
	dir := "ASC"
	if f.Desc {
		dir = "DESC"
	}
	offset := (f.Pagina - 1) * f.PorPagina

	consulta := fmt.Sprintf(`
		SELECT U.MATRICULA, U.NOMBRE, U.TIPO, U.GRUPO, U.ACTIVO
		FROM USUARIOS U%s
		ORDER BY %s %s, U.MATRICULA
		OFFSET %d ROWS FETCH NEXT %d ROWS ONLY`, where, col, dir, offset, f.PorPagina)

	filas, err := r.DB.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, fmt.Errorf("listando usuarios: %w", err)
	}
	defer filas.Close()

	porMatricula := make(map[string]*modelos.Usuario, f.PorPagina)
	for filas.Next() {
		u := &modelos.Usuario{Roles: []string{}}
		var grupo sql.NullString
		if err := filas.Scan(&u.Matricula, &u.Nombre, &u.Tipo, &grupo, &u.Activo); err != nil {
			return nil, err
		}
		limpiar(u)
		u.Protegido = u.Matricula == r.AdminClave
		if grupo.Valid && strings.TrimSpace(grupo.String) != "" {
			g := strings.TrimSpace(grupo.String)
			u.Grupo = &g
		}
		res.Items = append(res.Items, u)
		porMatricula[u.Matricula] = u
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}

	// Roles de los usuarios de ESTA página en una sola consulta
	if err := r.rolesDe(ctx, porMatricula); err != nil {
		return nil, err
	}
	return res, nil
}

// rolesDe llena Roles de cada usuario (los modifica por puntero)
func (r *UsuarioRepo) rolesDe(ctx context.Context, usuarios map[string]*modelos.Usuario) error {
	if len(usuarios) == 0 {
		return nil
	}
	marcas := make([]string, 0, len(usuarios))
	args := make([]any, 0, len(usuarios))
	for m := range usuarios {
		marcas = append(marcas, "?")
		args = append(args, m)
	}
	filas, err := r.DB.QueryContext(ctx, `
		SELECT MATRICULA, ROL FROM USUARIO_ROLES
		WHERE MATRICULA IN (`+strings.Join(marcas, ",")+`)
		ORDER BY CASE ROL WHEN 'ADMIN' THEN 1 WHEN 'PROFESOR' THEN 2 ELSE 3 END`, args...)
	if err != nil {
		return fmt.Errorf("leyendo roles: %w", err)
	}
	defer filas.Close()
	for filas.Next() {
		var m, rol string
		if err := filas.Scan(&m, &rol); err != nil {
			return err
		}
		if u, ok := usuarios[strings.TrimSpace(m)]; ok {
			u.Roles = append(u.Roles, strings.TrimSpace(rol))
		}
	}
	return filas.Err()
}

// ---------------------------------------------------------------------
// Resumen: tarjetas de arriba + lista de grupos para el filtro
// ---------------------------------------------------------------------

type ResumenUsuarios struct {
	Usuarios   int      `json:"usuarios"`
	Profesores int      `json:"profesores"`
	Admins     int      `json:"admins"`
	Inactivos  int      `json:"inactivos"`
	Grupos     []string `json:"grupos"`
}

func (r *UsuarioRepo) Resumen(ctx context.Context) (*ResumenUsuarios, error) {
	res := &ResumenUsuarios{Grupos: []string{}}
	err := r.DB.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM USUARIOS),
		  (SELECT COUNT(*) FROM USUARIO_ROLES WHERE ROL = 'PROFESOR'),
		  (SELECT COUNT(*) FROM USUARIO_ROLES WHERE ROL = 'ADMIN'),
		  (SELECT COUNT(*) FROM USUARIOS WHERE ACTIVO = FALSE)
		FROM RDB$DATABASE`).Scan(&res.Usuarios, &res.Profesores, &res.Admins, &res.Inactivos)
	if err != nil {
		return nil, fmt.Errorf("resumen de usuarios: %w", err)
	}

	filas, err := r.DB.QueryContext(ctx, `SELECT CODIGO FROM GRUPOS ORDER BY CODIGO`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	for filas.Next() {
		var g string
		if err := filas.Scan(&g); err != nil {
			return nil, err
		}
		res.Grupos = append(res.Grupos, strings.TrimSpace(g))
	}
	return res, filas.Err()
}

// ---------------------------------------------------------------------
// Editar roles / activo
// ---------------------------------------------------------------------

var (
	ErrNoEncontrado = errors.New("usuario no encontrado")
	ErrUltimoAdmin  = errors.New("debe quedar al menos un admin activo")
	ErrProtegido    = errors.New("el administrador general no se puede modificar")
)

// ActualizarRoles reemplaza los roles (ALUMNO siempre se conserva) y el estado activo
func (r *UsuarioRepo) ActualizarRoles(ctx context.Context, matricula string, roles []string, activo bool) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existe int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM USUARIOS WHERE MATRICULA = ?`, matricula).Scan(&existe); err != nil {
		return err
	}
	if existe == 0 {
		return ErrNoEncontrado
	}
	if matricula == r.AdminClave {
		return ErrProtegido
	}

	// Si le quitan ADMIN o lo desactivan, ¿queda otro admin activo?
	quedaAdmin := activo && contiene(roles, "ADMIN")
	if !quedaAdmin {
		var otros int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM USUARIOS U
			WHERE U.ACTIVO = TRUE AND U.MATRICULA <> ?
			  AND EXISTS (SELECT 1 FROM USUARIO_ROLES R WHERE R.MATRICULA = U.MATRICULA AND R.ROL = 'ADMIN')`,
			matricula).Scan(&otros); err != nil {
			return err
		}
		if otros == 0 {
			return ErrUltimoAdmin
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM USUARIO_ROLES WHERE MATRICULA = ?`, matricula); err != nil {
		return err
	}
	for _, rol := range roles {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO USUARIO_ROLES (MATRICULA, ROL) VALUES (?, ?)`, matricula, rol); err != nil {
			return fmt.Errorf("asignando %s: %w", rol, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE USUARIOS SET ACTIVO = ?, ACTUALIZADO = CURRENT_TIMESTAMP WHERE MATRICULA = ?`,
		activo, matricula); err != nil {
		return err
	}
	return tx.Commit()
}

// CambiarPassword guarda el hash nuevo (el hash se calcula en el handler)
func (r *UsuarioRepo) CambiarPassword(ctx context.Context, matricula, hash string) error {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE USUARIOS SET PASSWORD_HASH = ?, ACTUALIZADO = CURRENT_TIMESTAMP WHERE MATRICULA = ?`,
		hash, matricula)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoEncontrado
	}
	return nil
}

func contiene(lista []string, v string) bool {
	for _, x := range lista {
		if x == v {
			return true
		}
	}
	return false
}
