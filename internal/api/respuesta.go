package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func responderJSON(w http.ResponseWriter, status int, datos any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(datos)
}

// responderError usa "message" porque así lo lee el frontend (mensajeError)
func responderError(w http.ResponseWriter, status int, mensaje string) {
	responderJSON(w, status, map[string]string{"message": mensaje})
}

// leerJSON decodifica el cuerpo en el struct apuntado por destino (máx. 1 MB)
func leerJSON(r *http.Request, destino any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(destino)
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
