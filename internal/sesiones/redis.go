// Package sesiones: Redis se usa SOLO para las sesiones de login.
package sesiones

import (
	"context"
	"fmt"
	"log"
	"time"

	"raccoon_lab/internal/config"

	"github.com/redis/go-redis/v9"
)

// Conectar devuelve UN cliente compartido (*redis.Client) con su propio pool.
func Conectar(cfg *config.Config) (*redis.Client, error) {
	cliente := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Direccion,
		Password: cfg.Redis.Password,
		DB:       0,
	})

	var ultimoErr error
	for i := 1; i <= 15; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ultimoErr = cliente.Ping(ctx).Err()
		cancel()
		if ultimoErr == nil {
			log.Printf("[REDIS] Conectado a %s", cfg.Redis.Direccion)
			return cliente, nil
		}
		log.Printf("[REDIS] Esperando a Redis (%d/15): %v", i, ultimoErr)
		time.Sleep(2 * time.Second)
	}
	cliente.Close()
	return nil, fmt.Errorf("no se pudo conectar a Redis: %w", ultimoErr)
}
