package labs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"raccoon_lab/internal/repositorio"
)

var (
	ErrNoPuedeInvitar   = errors.New("para invitar necesitas ser dueño de un laboratorio colaborativo")
	ErrInvitacionNoVale = errors.New("la invitación no existe, ya fue respondida o el laboratorio se cerró")
)

// Companeros: alumnos de mi grupo (sin grupo -> lista vacía)
func (s *Servicio) Companeros(ctx context.Context, matricula string, grupo *string) ([]*repositorio.Companero, error) {
	if grupo == nil || *grupo == "" {
		return []*repositorio.Companero{}, nil
	}
	return s.Repo.Companeros(ctx, *grupo, matricula)
}

// Invitar: el dueño de un laboratorio colaborativo invita a compañeros de su grupo
func (s *Servicio) Invitar(ctx context.Context, matricula string, grupo *string, invitados []string) (int, error) {
	w, soyDueno, err := s.Repo.DeUsuario(ctx, matricula)
	if err != nil {
		return 0, err
	}
	if w == nil || !soyDueno || w.CodigoColab == nil {
		return 0, ErrNoPuedeInvitar
	}

	// Solo se puede invitar a compañeros del grupo que no estén en otro lab
	companeros, err := s.Companeros(ctx, matricula, grupo)
	if err != nil {
		return 0, err
	}
	enviadas := 0
	for _, m := range invitados {
		i := slices.IndexFunc(companeros, func(c *repositorio.Companero) bool { return c.Matricula == m })
		if i < 0 {
			return enviadas, fmt.Errorf("%s no es compañero de tu grupo", m)
		}
		if companeros[i].Ocupado {
			return enviadas, fmt.Errorf("%s ya está en un laboratorio", companeros[i].Nombre)
		}
		id := aleatorio(16, "abcdefghijkmnpqrstuvwxyz23456789")
		if err := s.Repo.CrearInvitacion(ctx, id, w.ID, matricula, m); err != nil {
			return enviadas, err
		}
		enviadas++
	}
	log.Printf("[LABS] %s invitó a %v a %s", matricula, invitados, w.ID)
	return enviadas, nil
}

// Pendientes: invitaciones que recibí
func (s *Servicio) Pendientes(ctx context.Context, matricula string) ([]*repositorio.Invitacion, error) {
	return s.Repo.Pendientes(ctx, matricula)
}

// Responder: aceptar = entrar al laboratorio del emisor; rechazar = solo marcarla
func (s *Servicio) Responder(ctx context.Context, matricula, host, idInvitacion string, aceptar bool) (*Estado, error) {
	inv, err := s.Repo.InvitacionPendiente(ctx, idInvitacion, matricula)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInvitacionNoVale
	}
	if !aceptar {
		return nil, s.Repo.MarcarInvitacion(ctx, inv.ID, "RECHAZADA")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if w, _, err := s.Repo.DeUsuario(ctx, matricula); err != nil {
		return nil, err
	} else if w != nil {
		return nil, ErrYaActivo
	}
	w, err := s.Repo.PorID(ctx, inv.IDWorkspace)
	if err != nil {
		return nil, err
	}
	if w == nil {
		_ = s.Repo.MarcarInvitacion(ctx, inv.ID, "EXPIRADA")
		return nil, ErrInvitacionNoVale
	}
	if err := s.Repo.AgregarMiembro(ctx, w.ID, matricula); err != nil {
		return nil, err
	}
	_ = s.Repo.MarcarInvitacion(ctx, inv.ID, "ACEPTADA")
	log.Printf("[LABS] %s aceptó la invitación de %s a %s", matricula, inv.Emisor, w.ID)

	corriendo, _ := s.Docker.Corriendo(ctx, w.ID)
	return s.estadoDe(w, false, host, corriendo), nil
}
