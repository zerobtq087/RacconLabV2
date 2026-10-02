package api

import (
	"context"
	"net/http"
	"time"
)

// Salud revisa que Firebird y Redis respondan
func (a *App) Salud(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	estadoDB, estadoRedis, estadoDocker := "ok", "ok", "ok"
	if err := a.DB.PingContext(ctx); err != nil {
		estadoDB = "error: " + err.Error()
	}
	if err := a.Redis.Ping(ctx).Err(); err != nil {
		estadoRedis = "error: " + err.Error()
	}
	// Docker no es indispensable para entrar a la plataforma (solo para los labs),
	// por eso un fallo aquí no cambia el estado general a "degradado"
	if v, err := a.Docker.Version(ctx); err != nil {
		estadoDocker = "error: " + err.Error()
	} else {
		estadoDocker = "ok (" + v + ")"
	}

	status, estado := http.StatusOK, "ok"
	if estadoDB != "ok" || estadoRedis != "ok" {
		status, estado = http.StatusServiceUnavailable, "degradado"
	}
	responderJSON(w, status, map[string]any{
		"estado":   estado,
		"version":  a.Version,
		"entorno":  a.Cfg.Entorno,
		"firebird": estadoDB,
		"redis":    estadoRedis,
		"docker":   estadoDocker,
		"activo_s": int(time.Since(a.Inicio).Seconds()),
	})
}
