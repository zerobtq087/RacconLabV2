package repositorio

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Workspace: un laboratorio GNS3 activo (un contenedor Docker)
type Workspace struct {
	ID          string // nombre del contenedor
	Dueno       string
	PuertoBase  int
	Token       string
	CodigoColab *string // nil = individual
	Inicio      time.Time
}

type WorkspaceRepo struct {
	DB *sql.DB
}

const columnasWS = `W.ID, W.DUENO, W.PUERTO_BASE, W.TOKEN, W.CODIGO_COLAB, W.INICIO`

type escaner interface{ Scan(...any) error }

func escanearWS(f escaner) (*Workspace, error) {
	w := &Workspace{}
	var codigo sql.NullString
	if err := f.Scan(&w.ID, &w.Dueno, &w.PuertoBase, &w.Token, &codigo, &w.Inicio); err != nil {
		return nil, err
	}
	w.ID, w.Dueno, w.Token = strings.TrimSpace(w.ID), strings.TrimSpace(w.Dueno), strings.TrimSpace(w.Token)
	if codigo.Valid && strings.TrimSpace(codigo.String) != "" {
		c := strings.TrimSpace(codigo.String)
		w.CodigoColab = &c
	}
	return w, nil
}

// DeUsuario: el laboratorio donde está el usuario (como dueño o como miembro).
// Regresa (nil, false, nil) si no está en ninguno.
func (r *WorkspaceRepo) DeUsuario(ctx context.Context, matricula string) (*Workspace, bool, error) {
	w, err := escanearWS(r.DB.QueryRowContext(ctx,
		`SELECT `+columnasWS+` FROM WORKSPACES W WHERE W.DUENO = ?`, matricula))
	if err == nil {
		return w, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	w, err = escanearWS(r.DB.QueryRowContext(ctx, `
		SELECT `+columnasWS+` FROM WORKSPACES W
		JOIN WORKSPACE_MIEMBROS M ON M.ID_WORKSPACE = W.ID
		WHERE M.MATRICULA = ?`, matricula))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return w, false, err
}

func (r *WorkspaceRepo) PorCodigo(ctx context.Context, codigo string) (*Workspace, error) {
	w, err := escanearWS(r.DB.QueryRowContext(ctx,
		`SELECT `+columnasWS+` FROM WORKSPACES W WHERE W.CODIGO_COLAB = ?`, codigo))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return w, err
}

func (r *WorkspaceRepo) Todos(ctx context.Context) ([]*Workspace, error) {
	filas, err := r.DB.QueryContext(ctx, `SELECT `+columnasWS+` FROM WORKSPACES W ORDER BY W.INICIO`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	lista := []*Workspace{}
	for filas.Next() {
		w, err := escanearWS(filas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, w)
	}
	return lista, filas.Err()
}

func (r *WorkspaceRepo) PuertosUsados(ctx context.Context) (map[int]bool, error) {
	filas, err := r.DB.QueryContext(ctx, `SELECT PUERTO_BASE FROM WORKSPACES`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	usados := map[int]bool{}
	for filas.Next() {
		var p int
		if err := filas.Scan(&p); err != nil {
			return nil, err
		}
		usados[p] = true
	}
	return usados, filas.Err()
}

func (r *WorkspaceRepo) CodigoExiste(ctx context.Context, codigo string) (bool, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM WORKSPACES WHERE CODIGO_COLAB = ?`, codigo).Scan(&n)
	return n > 0, err
}

func (r *WorkspaceRepo) Guardar(ctx context.Context, w *Workspace) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO WORKSPACES (ID, DUENO, PUERTO_BASE, TOKEN, CODIGO_COLAB)
		VALUES (?, ?, ?, ?, ?)`, w.ID, w.Dueno, w.PuertoBase, w.Token, w.CodigoColab)
	return err
}

// Eliminar borra el workspace; miembros e invitaciones se van en cascada
func (r *WorkspaceRepo) Eliminar(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM WORKSPACES WHERE ID = ?`, id)
	return err
}

func (r *WorkspaceRepo) AgregarMiembro(ctx context.Context, id, matricula string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO WORKSPACE_MIEMBROS (ID_WORKSPACE, MATRICULA) VALUES (?, ?)`, id, matricula)
	return err
}

func (r *WorkspaceRepo) QuitarMiembro(ctx context.Context, matricula string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM WORKSPACE_MIEMBROS WHERE MATRICULA = ?`, matricula)
	return err
}
