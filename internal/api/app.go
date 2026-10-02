// Package api: handlers HTTP. Todos son métodos de *App (sin variables globales).
package api

import (
	"database/sql"
	"net/http"
	"time"

	"raccoon_lab/internal/config"
	"raccoon_lab/internal/repositorio"
	"raccoon_lab/internal/seguridad"
	"raccoon_lab/internal/sesiones"

	"github.com/redis/go-redis/v9"
)

// App agrupa TODAS las dependencias. Se crea una sola vez y se comparte por puntero.
type App struct {
	Cfg       *config.Config
	DB        *sql.DB
	Redis     *redis.Client
	Sesiones  *sesiones.Store
	Usuarios  *repositorio.UsuarioRepo
	Limitador *seguridad.Limitador
	Version   string
	Inicio    time.Time
}

func NuevaApp(cfg *config.Config, db *sql.DB, rdb *redis.Client, version string) *App {
	return &App{
		Cfg:       cfg,
		DB:        db,
		Redis:     rdb,
		Sesiones:  &sesiones.Store{R: rdb, Duracion: cfg.DuracionSesion},
		Usuarios:  &repositorio.UsuarioRepo{DB: db},
		Limitador: seguridad.NuevoLimitador(5, 15*time.Minute),
		Version:   version,
		Inicio:    time.Now(),
	}
}

// Rutas registra todos los endpoints
func (a *App) Rutas() http.Handler {
	mux := http.NewServeMux()

	// Públicas
	mux.HandleFunc("GET /api/salud", a.Salud)
	mux.HandleFunc("POST /api/auth/login", a.Login)
	mux.HandleFunc("POST /api/auth/refresh", a.Refresh)
	mux.HandleFunc("POST /api/auth/logout", a.Logout)

	// Con sesión (cualquier rol)
	mux.Handle("POST /api/auth/cambiar-rol", a.ConSesion(a.CambiarRol))

	// API que aún no existe -> 404 JSON
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		responderError(w, http.StatusNotFound, "Ruta de API no encontrada")
	})

	// Frontend (Vue)
	mux.Handle("/", frontend(a.Cfg.WebDir))
	return mux
}
