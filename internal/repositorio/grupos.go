package repositorio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// GrupoRepo: grupos y maestros asignados. Comparte el *sql.DB de la app.
type GrupoRepo struct {
	DB *sql.DB
}

type Docente struct {
	Matricula string `json:"matricula"`
	Nombre    string `json:"nombre"`
	Activo    bool   `json:"activo"` // false = inactivo o ya sin rol Maestro
}

type Grupo struct {
	Codigo       string     `json:"codigo"`
	TotalAlumnos int        `json:"total_alumnos"`
	Docentes     []*Docente `json:"docentes"`
}

type AlumnoGrupo struct {
	Matricula string `json:"matricula"`
	Nombre    string `json:"nombre"`
	Activo    bool   `json:"activo"`
}

type MaestroDisponible struct {
	Matricula string `json:"matricula"`
	Nombre    string `json:"nombre"`
}

var (
	ErrGrupoNoExiste  = errors.New("el grupo no existe")
	ErrGrupoExiste    = errors.New("ya existe un grupo con ese código")
	ErrGrupoConAlumno = errors.New("el grupo tiene alumnos")
)

// Listar: si docente != "" solo regresa los grupos donde está asignado
func (r *GrupoRepo) Listar(ctx context.Context, docente string) ([]*Grupo, error) {
	consulta := `
		SELECT G.CODIGO, (SELECT COUNT(*) FROM USUARIOS U WHERE U.GRUPO = G.CODIGO)
		FROM GRUPOS G`
	var args []any
	if docente != "" {
		consulta += ` WHERE EXISTS (SELECT 1 FROM GRUPO_DOCENTES D WHERE D.CODIGO = G.CODIGO AND D.MATRICULA = ?)`
		args = append(args, docente)
	}
	consulta += ` ORDER BY G.CODIGO`

	filas, err := r.DB.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, fmt.Errorf("listando grupos: %w", err)
	}
	defer filas.Close()

	grupos := []*Grupo{}
	porCodigo := map[string]*Grupo{}
	for filas.Next() {
		g := &Grupo{Docentes: []*Docente{}}
		if err := filas.Scan(&g.Codigo, &g.TotalAlumnos); err != nil {
			return nil, err
		}
		g.Codigo = strings.TrimSpace(g.Codigo)
		grupos = append(grupos, g)
		porCodigo[g.Codigo] = g
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if len(grupos) == 0 {
		return grupos, nil
	}

	// Maestros de todos los grupos en una sola consulta; se cuelgan por puntero
	filas2, err := r.DB.QueryContext(ctx, `
		SELECT D.CODIGO, U.MATRICULA, U.NOMBRE,
		       CASE WHEN U.ACTIVO AND EXISTS (SELECT 1 FROM USUARIO_ROLES R
		            WHERE R.MATRICULA = U.MATRICULA AND R.ROL = 'PROFESOR') THEN 1 ELSE 0 END
		FROM GRUPO_DOCENTES D JOIN USUARIOS U ON U.MATRICULA = D.MATRICULA
		ORDER BY U.NOMBRE`)
	if err != nil {
		return nil, fmt.Errorf("leyendo maestros de grupos: %w", err)
	}
	defer filas2.Close()
	for filas2.Next() {
		var codigo string
		var activo int
		d := &Docente{}
		if err := filas2.Scan(&codigo, &d.Matricula, &d.Nombre, &activo); err != nil {
			return nil, err
		}
		if g, ok := porCodigo[strings.TrimSpace(codigo)]; ok {
			d.Matricula, d.Nombre, d.Activo = strings.TrimSpace(d.Matricula), strings.TrimSpace(d.Nombre), activo == 1
			g.Docentes = append(g.Docentes, d)
		}
	}
	return grupos, filas2.Err()
}

// EsDocente: ¿el maestro está asignado a ese grupo?
func (r *GrupoRepo) EsDocente(ctx context.Context, codigo, matricula string) (bool, error) {
	var n int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM GRUPO_DOCENTES WHERE CODIGO = ? AND MATRICULA = ?`, codigo, matricula).Scan(&n)
	return n > 0, err
}

func (r *GrupoRepo) Existe(ctx context.Context, codigo string) (bool, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM GRUPOS WHERE CODIGO = ?`, codigo).Scan(&n)
	return n > 0, err
}

func (r *GrupoRepo) Alumnos(ctx context.Context, codigo string) ([]*AlumnoGrupo, error) {
	filas, err := r.DB.QueryContext(ctx, `
		SELECT MATRICULA, NOMBRE, ACTIVO FROM USUARIOS
		WHERE GRUPO = ? ORDER BY NOMBRE`, codigo)
	if err != nil {
		return nil, fmt.Errorf("alumnos de %s: %w", codigo, err)
	}
	defer filas.Close()
	lista := []*AlumnoGrupo{}
	for filas.Next() {
		a := &AlumnoGrupo{}
		if err := filas.Scan(&a.Matricula, &a.Nombre, &a.Activo); err != nil {
			return nil, err
		}
		a.Matricula, a.Nombre = strings.TrimSpace(a.Matricula), strings.TrimSpace(a.Nombre)
		lista = append(lista, a)
	}
	return lista, filas.Err()
}

// MaestrosDisponibles: usuarios activos con rol PROFESOR
func (r *GrupoRepo) MaestrosDisponibles(ctx context.Context) ([]*MaestroDisponible, error) {
	filas, err := r.DB.QueryContext(ctx, `
		SELECT U.MATRICULA, U.NOMBRE FROM USUARIOS U
		WHERE U.ACTIVO = TRUE
		  AND EXISTS (SELECT 1 FROM USUARIO_ROLES R WHERE R.MATRICULA = U.MATRICULA AND R.ROL = 'PROFESOR')
		ORDER BY U.NOMBRE`)
	if err != nil {
		return nil, fmt.Errorf("maestros disponibles: %w", err)
	}
	defer filas.Close()
	lista := []*MaestroDisponible{}
	for filas.Next() {
		m := &MaestroDisponible{}
		if err := filas.Scan(&m.Matricula, &m.Nombre); err != nil {
			return nil, err
		}
		m.Matricula, m.Nombre = strings.TrimSpace(m.Matricula), strings.TrimSpace(m.Nombre)
		lista = append(lista, m)
	}
	return lista, filas.Err()
}

func (r *GrupoRepo) Crear(ctx context.Context, codigo string) error {
	existe, err := r.Existe(ctx, codigo)
	if err != nil {
		return err
	}
	if existe {
		return ErrGrupoExiste
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO GRUPOS (CODIGO) VALUES (?)`, codigo)
	return err
}

// Eliminar: solo si no tiene alumnos (para no dejarlos sin grupo por accidente)
func (r *GrupoRepo) Eliminar(ctx context.Context, codigo string) (int, error) {
	var alumnos int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM USUARIOS WHERE GRUPO = ?`, codigo).Scan(&alumnos); err != nil {
		return 0, err
	}
	if alumnos > 0 {
		return alumnos, ErrGrupoConAlumno
	}
	res, err := r.DB.ExecContext(ctx, `DELETE FROM GRUPOS WHERE CODIGO = ?`, codigo)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrGrupoNoExiste
	}
	return 0, nil
}

// AsignarDocentes reemplaza la lista completa de maestros del grupo.
// Regresa las matrículas que no son maestros activos (no se guarda nada si hay alguna).
func (r *GrupoRepo) AsignarDocentes(ctx context.Context, codigo string, docentes []string) ([]string, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM GRUPOS WHERE CODIGO = ?`, codigo).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrGrupoNoExiste
	}

	var invalidos []string
	for _, m := range docentes {
		var ok int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM USUARIOS U
			WHERE U.MATRICULA = ? AND U.ACTIVO = TRUE
			  AND EXISTS (SELECT 1 FROM USUARIO_ROLES R WHERE R.MATRICULA = U.MATRICULA AND R.ROL = 'PROFESOR')`,
			m).Scan(&ok); err != nil {
			return nil, err
		}
		if ok == 0 {
			invalidos = append(invalidos, m)
		}
	}
	if len(invalidos) > 0 {
		return invalidos, nil
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM GRUPO_DOCENTES WHERE CODIGO = ?`, codigo); err != nil {
		return nil, err
	}
	for _, m := range docentes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO GRUPO_DOCENTES (CODIGO, MATRICULA) VALUES (?, ?)`, codigo, m); err != nil {
			return nil, fmt.Errorf("asignando %s: %w", m, err)
		}
	}
	return nil, tx.Commit()
}
