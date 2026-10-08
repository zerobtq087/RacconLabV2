package repositorio

import (
	"context"
	"strings"
)

// MiembrosPorWorkspace: id del workspace -> matrículas de los invitados (una sola consulta)
func (r *WorkspaceRepo) MiembrosPorWorkspace(ctx context.Context) (map[string][]string, error) {
	filas, err := r.DB.QueryContext(ctx,
		`SELECT ID_WORKSPACE, MATRICULA FROM WORKSPACE_MIEMBROS ORDER BY UNIDO`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	mapa := map[string][]string{}
	for filas.Next() {
		var id, m string
		if err := filas.Scan(&id, &m); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		mapa[id] = append(mapa[id], strings.TrimSpace(m))
	}
	return mapa, filas.Err()
}
