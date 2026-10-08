package labs

import (
	"context"
	"log"
	"time"
)

// Resumen de un laboratorio para el monitor del admin
type Resumen struct {
	IDWorkspace string    `json:"id_workspace"`
	CreadoPor   string    `json:"creado_por"`
	Miembros    []string  `json:"miembros"`
	PuertoBase  int       `json:"puerto_base"`
	CodigoLab   *string   `json:"codigo_lab"`
	FechaInicio time.Time `json:"fecha_inicio"`
	Running     bool      `json:"running"`
}

// Todos: los laboratorios registrados con sus miembros y si el contenedor corre
func (s *Servicio) Todos(ctx context.Context) ([]*Resumen, error) {
	lista, err := s.Repo.Todos(ctx)
	if err != nil {
		return nil, err
	}
	miembros, err := s.Repo.MiembrosPorWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*Resumen, 0, len(lista))
	for _, w := range lista {
		corriendo, _ := s.Docker.Corriendo(ctx, w.ID)
		m := miembros[w.ID]
		if m == nil {
			m = []string{}
		}
		res = append(res, &Resumen{
			IDWorkspace: w.ID,
			CreadoPor:   w.Dueno,
			Miembros:    m,
			PuertoBase:  w.PuertoBase,
			CodigoLab:   w.CodigoColab,
			FechaInicio: w.Inicio,
			Running:     corriendo,
		})
	}
	return res, nil
}

// Cerrar: el admin destruye un laboratorio (para todo el equipo)
func (s *Servicio) Cerrar(ctx context.Context, id, admin string) error {
	w, err := s.Repo.PorID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return ErrSinLaboratorio
	}
	s.destruir(ctx, w)
	log.Printf("[LABS] el admin %s cerró %s (dueño %s)", admin, w.ID, w.Dueno)
	return nil
}
