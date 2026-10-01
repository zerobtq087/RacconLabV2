// Raccoon Lab v2 - servidor principal
// Por ahora: sirve el frontend (Vue) y /api/salud.
// En los siguientes pasos se agregan login, Firebird, Redis, GNS3, etc.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var version = "dev" // se reemplaza al compilar con -ldflags "-X main.version=..."

func env(clave, porDefecto string) string {
	if v := strings.TrimSpace(os.Getenv(clave)); v != "" {
		return v
	}
	return porDefecto
}

// frontend sirve archivos estáticos y regresa index.html para las rutas de Vue (SPA)
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
		// Ruta de Vue (ej. /admin/usuarios) -> index.html sin caché
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func responderJSON(w http.ResponseWriter, status int, datos any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(datos)
}

func main() {
	puerto := env("PORT", "8080")
	webDir := env("WEB_DIR", "/app/web")
	inicio := time.Now()

	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		log.Printf("[AVISO] No existe %s/index.html; el frontend no se mostrará", webDir)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/salud", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, map[string]any{
			"estado":   "ok",
			"version":  version,
			"entorno":  env("ENV", "desconocido"),
			"activo_s": int(time.Since(inicio).Seconds()),
		})
	})

	// Cualquier /api/ que aún no exista -> 404 en JSON (no index.html)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusNotFound, map[string]string{"error": "Ruta de API no encontrada"})
	})

	mux.Handle("/", frontend(webDir))

	srv := &http.Server{
		Addr:              ":" + puerto,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Raccoon Lab %s escuchando en :%s (frontend: %s)", version, puerto, webDir)
	log.Fatal(srv.ListenAndServe())
}
