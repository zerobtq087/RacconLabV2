// Raccoon Lab v2 - servidor principal
// Firebird = todos los datos | Redis = solo sesiones
// Todas las dependencias se crean una vez y se comparten por PUNTERO.
package main

import (
	"log"
	"net/http"
	"time"

	"raccoon_lab/internal/api"
	"raccoon_lab/internal/config"
	"raccoon_lab/internal/database"
	"raccoon_lab/internal/sesiones"
)

var version = "dev"

func main() {
	log.Printf("=== Raccoon Lab %s ===", version)

	cfg, err := config.Cargar()
	if err != nil {
		log.Fatalf("[CONFIG] %v", err)
	}

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

	rdb, err := sesiones.Conectar(cfg)
	if err != nil {
		log.Fatalf("[REDIS] %v", err)
	}
	defer rdb.Close()

	app := api.NuevaApp(cfg, db, rdb, version)

	srv := &http.Server{
		Addr:              ":" + cfg.Puerto,
		Handler:           app.Rutas(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Escuchando en :%s (entorno: %s)", cfg.Puerto, cfg.Entorno)
	log.Fatal(srv.ListenAndServe())
}
