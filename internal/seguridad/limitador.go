// Package seguridad: protección contra fuerza bruta en el login.
// Vive en la memoria de la app (Redis es solo para sesiones).
package seguridad

import (
	"sync"
	"time"
)

type intento struct {
	fallos       int
	bloqueoHasta time.Time
	ultimo       time.Time
}

// Limitador cuenta intentos fallidos por matrícula.
// El mapa guarda punteros: se actualiza el registro original sin copiarlo.
type Limitador struct {
	mu        sync.Mutex
	intentos  map[string]*intento
	MaxFallos int
	Bloqueo   time.Duration
}

func NuevoLimitador(maxFallos int, bloqueo time.Duration) *Limitador {
	l := &Limitador{
		intentos:  make(map[string]*intento),
		MaxFallos: maxFallos,
		Bloqueo:   bloqueo,
	}
	go l.limpiarPeriodicamente()
	return l
}

// Bloqueado indica si la matrícula está bloqueada y por cuánto tiempo más
func (l *Limitador) Bloqueado(clave string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r, ok := l.intentos[clave]; ok && time.Now().Before(r.bloqueoHasta) {
		return true, time.Until(r.bloqueoHasta).Round(time.Second)
	}
	return false, 0
}

// Fallo registra un intento fallido; al llegar a MaxFallos bloquea
func (l *Limitador) Fallo(clave string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.intentos[clave]
	if !ok {
		r = &intento{}
		l.intentos[clave] = r
	}
	r.fallos++
	r.ultimo = time.Now()
	if r.fallos >= l.MaxFallos {
		r.bloqueoHasta = time.Now().Add(l.Bloqueo)
		r.fallos = 0
	}
}

// Exito borra el registro de la matrícula
func (l *Limitador) Exito(clave string) {
	l.mu.Lock()
	delete(l.intentos, clave)
	l.mu.Unlock()
}

// limpiarPeriodicamente evita que el mapa crezca sin límite
func (l *Limitador) limpiarPeriodicamente() {
	for range time.Tick(10 * time.Minute) {
		l.mu.Lock()
		for k, r := range l.intentos {
			if time.Since(r.ultimo) > time.Hour && time.Now().After(r.bloqueoHasta) {
				delete(l.intentos, k)
			}
		}
		l.mu.Unlock()
	}
}
