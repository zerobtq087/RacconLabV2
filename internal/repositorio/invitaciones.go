package repositorio

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Companero: alumno del mismo grupo (para invitar)
type Companero struct {
	Matricula string `json:"matricula"`
	Nombre    string `json:"nombre"`
	Ocupado   bool   `json:"ocupado"` // ya está en un laboratorio
}

// Invitacion pendiente que ve el receptor
type Invitacion struct {
	ID           string `json:"id_invitacion"`
	IDWorkspace  string `json:"id_workspace"`
	Emisor       string `json:"emisor"`
	NombreEmisor string `json:"nombre_emisor"`
}

// Companeros del grupo (activos, sin contarme). Ocupado = dueño o miembro de un lab.
func (r *WorkspaceRepo) Companeros(ctx context.Context, grupo, yo string) ([]*Companero, error) {
	filas, err := r.DB.QueryContext(ctx, `
		SELECT U.MATRICULA, U.NOMBRE,
		       CASE WHEN EXISTS (SELECT 1 FROM WORKSPACES W WHERE W.DUENO = U.MATRICULA)
		              OR EXISTS (SELECT 1 FROM WORKSPACE_MIEMBROS M WHERE M.MATRICULA = U.MATRICULA)
		            THEN 1 ELSE 0 END
		FROM USUARIOS U
		WHERE U.GRUPO = ? AND U.MATRICULA <> ? AND U.ACTIVO = TRUE
		ORDER BY U.NOMBRE`, grupo, yo)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	lista := []*Companero{}
	for filas.Next() {
		c := &Companero{}
		var ocupado int
		if err := filas.Scan(&c.Matricula, &c.Nombre, &ocupado); err != nil {
			return nil, err
		}
		c.Matricula, c.Nombre, c.Ocupado = strings.TrimSpace(c.Matricula), strings.TrimSpace(c.Nombre), ocupado == 1
		lista = append(lista, c)
	}
	return lista, filas.Err()
}

// CrearInvitacion: si ya hay una pendiente para ese lab y receptor, no duplica
func (r *WorkspaceRepo) CrearInvitacion(ctx context.Context, id, idWorkspace, emisor, receptor string) error {
	var n int
	if err := r.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INVITACIONES
		WHERE ID_WORKSPACE = ? AND RECEPTOR = ? AND ESTADO = 'PENDIENTE'`, idWorkspace, receptor).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO INVITACIONES (ID, ID_WORKSPACE, EMISOR, RECEPTOR) VALUES (?, ?, ?, ?)`,
		id, idWorkspace, emisor, receptor)
	return err
}

func (r *WorkspaceRepo) Pendientes(ctx context.Context, receptor string) ([]*Invitacion, error) {
	filas, err := r.DB.QueryContext(ctx, `
		SELECT I.ID, I.ID_WORKSPACE, I.EMISOR, U.NOMBRE
		FROM INVITACIONES I JOIN USUARIOS U ON U.MATRICULA = I.EMISOR
		WHERE I.RECEPTOR = ? AND I.ESTADO = 'PENDIENTE'
		ORDER BY I.CREADA DESC`, receptor)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	lista := []*Invitacion{}
	for filas.Next() {
		i := &Invitacion{}
		if err := filas.Scan(&i.ID, &i.IDWorkspace, &i.Emisor, &i.NombreEmisor); err != nil {
			return nil, err
		}
		i.ID, i.IDWorkspace = strings.TrimSpace(i.ID), strings.TrimSpace(i.IDWorkspace)
		i.Emisor, i.NombreEmisor = strings.TrimSpace(i.Emisor), strings.TrimSpace(i.NombreEmisor)
		lista = append(lista, i)
	}
	return lista, filas.Err()
}

// InvitacionPendiente: la invitación si es del receptor y sigue pendiente (nil si no)
func (r *WorkspaceRepo) InvitacionPendiente(ctx context.Context, id, receptor string) (*Invitacion, error) {
	i := &Invitacion{}
	err := r.DB.QueryRowContext(ctx, `
		SELECT ID, ID_WORKSPACE, EMISOR FROM INVITACIONES
		WHERE ID = ? AND RECEPTOR = ? AND ESTADO = 'PENDIENTE'`, id, receptor).Scan(&i.ID, &i.IDWorkspace, &i.Emisor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i.ID, i.IDWorkspace, i.Emisor = strings.TrimSpace(i.ID), strings.TrimSpace(i.IDWorkspace), strings.TrimSpace(i.Emisor)
	return i, nil
}

func (r *WorkspaceRepo) MarcarInvitacion(ctx context.Context, id, estado string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE INVITACIONES SET ESTADO = ? WHERE ID = ?`, estado, id)
	return err
}

// PorID: workspace por su id (nil si ya no existe)
func (r *WorkspaceRepo) PorID(ctx context.Context, id string) (*Workspace, error) {
	w, err := escanearWS(r.DB.QueryRowContext(ctx,
		`SELECT `+columnasWS+` FROM WORKSPACES W WHERE W.ID = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return w, err
}
