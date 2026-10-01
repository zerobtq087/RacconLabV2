// Raccoon Lab v2 - servidor principal
// Firebird = todos los datos | Redis = solo sesiones
// Todas las dependencias se comparten por PUNTERO (sin copias, sin globales).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"raccoon_lab/internal/config"
	"raccoon_lab/internal/database"
	"raccoon_lab/internal/sesiones"

	"github.com/redis/go-redis/v9"
)

var version = "dev" // se reemplaza al compilar con -ldflags "-X main.version=..."

// App agrupa las dependencias. Se crea una vez y los handlers son métodos (*App).
type App struct {
	Cfg    *config.Config
	DB     *sql.DB
	Redis  *redis.Client
	Inicio time.Time
}

func responderJSON(w http.ResponseWriter, status int, datos any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(datos)
}

// Salud revisa que Firebird y Redis respondan
func (a *App) Salud(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	estadoDB, estadoRedis := "ok", "ok"
	if err := a.DB.PingContext(ctx); err != nil {
		estadoDB = "error: " + err.Error()
	}
	if err := a.Redis.Ping(ctx).Err(); err != nil {
		estadoRedis = "error: " + err.Error()
	}

	status := http.StatusOK
	if estadoDB != "ok" || estadoRedis != "ok" {
		status = http.StatusServiceUnavailable
	}
	responderJSON(w, status, map[string]any{
		"estado":   map[bool]string{true: "ok", false: "degradado"}[status == http.StatusOK],
		"version":  version,
		"entorno":  a.Cfg.Entorno,
		"firebird": estadoDB,
		"redis":    estadoRedis,
		"activo_s": int(time.Since(a.Inicio).Seconds()),
	})
}

// frontend sirve el build de Vue y regresa index.html para las rutas de la SPA
func frontend(dir string) http.Handler {
	archivos := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ruta := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(ruta); err == nil && !info.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			archivos.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func main() {
	log.Printf("=== Raccoon Lab %s ===", version)

	// 1. Configuración (una sola instancia, compartida por puntero)
	cfg, err := config.Cargar()
	if err != nil {
		log.Fatalf("[CONFIG] %v", err)
	}

	// 2. Firebird: conexión, esquema y admin inicial
	db, err := database.Conectar(cfg)
	if err != nil {
		log.Fatalf("[BD] %v", err)
	}
	defer db.Close()

	if err := database.AplicarEsquema(db); err != nil {
		log.Fatalf("[BD] %v", err)
	}
	if err := database.CrearAdminInicial(db, cfg); err != nil {
		log.Fatalf("[BD] %v", err)
	}

	// 3. Redis (solo sesiones)
	rdb, err := sesiones.Conectar(cfg)
	if err != nil {
		log.Fatalf("[REDIS] %v", err)
	}
	defer rdb.Close()

	// 4. App con todas las dependencias por puntero
	app := &App{Cfg: cfg, DB: db, Redis: rdb, Inicio: time.Now()}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/salud", app.Salud)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusNotFound, map[string]string{"error": "Ruta de API no encontrada"})
	})
	mux.Handle("/", frontend(cfg.WebDir))

	srv := &http.Server{
		Addr:              ":" + cfg.Puerto,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Escuchando en :%s (entorno: %s)", cfg.Puerto, cfg.Entorno)
	log.Fatal(srv.ListenAndServe())
}
