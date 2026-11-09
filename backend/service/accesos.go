package service

import (
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
)

// Accesos arma la lista del firewall para el agente raccoon-accesos.
//
// Todos los puertos de un lab están cerrados por defecto. A cada IP con sesión
// web vigente y acceso al lab (dueño, equipo o docente supervisando) se le abre:
//   - el puerto del GNS3 del lab (para el GNS3 de escritorio), y
//   - SOLO los puertos de consola de los nodos que existen en sus proyectos
//     abiertos. Al agregar un nodo su puerto aparece en la siguiente consulta;
//     al borrarlo, desaparece.
//
// Los nodos se preguntan al GNS3 de cada lab en cada consulta (el agente
// pregunta cada segundo), así que no hay estado guardado: las 2 réplicas dan
// la misma lista y sobrevive a reinicios. "ultimos" es solo un respaldo por
// réplica: si el GNS3 de un lab tarda en contestar, se usan sus últimos
// puertos en vez de cerrar las consolas que el alumno tiene abiertas.
type Accesos struct {
	repo *repository.AccesoRepository
	env  *config.Env

	mu      sync.Mutex
	ultimos map[string][]int // ID del lab → últimos puertos de consola conocidos
	latido  time.Time        // última vez que se anotó el latido del agente
}

func NewAccesos(r *repository.AccesoRepository, env *config.Env) *Accesos {
	return &Accesos{repo: r, env: env, ultimos: map[string][]int{}}
}

// Lista: una línea "IP . puerto" por cada puerto abierto, sin repetir y ordenada
func (s *Accesos) Lista() []string {
	s.anotarLatido()

	// Mismos tiempos que la sesión en Redis (cmd/main.go): 8 h sin uso o 12 h en total
	accesos := s.repo.Accesos(8*time.Hour, 12*time.Hour)

	// Los puertos de consola de cada lab, preguntando a todos los GNS3 a la vez
	labs := map[string]domain.Acceso{}
	for _, a := range accesos {
		if a.Estado == "CORRIENDO" {
			labs[a.IDLab] = a
		}
	}
	consolas := map[string][]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for id, a := range labs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			puertos := s.consolas(id, a.Puerto)
			mu.Lock()
			consolas[id] = puertos
			mu.Unlock()
		}()
	}
	wg.Wait()

	vistas := map[string]bool{}
	lineas := []string{}
	agregar := func(ip netip.Addr, puerto int) {
		l := fmt.Sprintf("%s . %d", ip, puerto)
		if !vistas[l] {
			vistas[l] = true
			lineas = append(lineas, l)
		}
	}
	for _, a := range accesos {
		ip, err := netip.ParseAddr(a.IP)
		if err != nil || !ip.Is4() {
			continue // el firewall de los labs es solo IPv4
		}
		agregar(ip, a.Puerto)
		for _, p := range consolas[a.IDLab] {
			agregar(ip, p)
		}
	}
	sort.Strings(lineas)
	return lineas
}

// consolas: los puertos de consola del lab (o los últimos conocidos si su GNS3 no contesta)
func (s *Accesos) consolas(idLab string, base int) []int {
	g := NewGNS3(s.env, &domain.Laboratorio{PuertoBase: base})
	puertos, err := g.PuertosConsola(base+1, base+s.env.LabPuertos-1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		return s.ultimos[idLab]
	}
	s.ultimos[idLab] = puertos
	return puertos
}

// anotarLatido: el agente pregunta cada segundo; en Firebird basta cada 10 s
// (la página de Inicio del admin dice si el agente está vivo)
func (s *Accesos) anotarLatido() {
	s.mu.Lock()
	toca := time.Since(s.latido) > 10*time.Second
	if toca {
		s.latido = time.Now()
	}
	s.mu.Unlock()
	if toca {
		s.repo.Latido("raccoon-accesos")
	}
}
