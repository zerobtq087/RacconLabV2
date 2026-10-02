// Package config lee las variables del .env (inyectadas por Podman).
// Se crea UNA sola vez y se comparte en toda la app como *Config.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type BaseDatos struct {
	Usuario  string
	Password string
	Host     string
	Puerto   int
	Ruta     string // ruta del .fdb DENTRO del contenedor de Firebird
}

type Redis struct {
	Direccion string
	Password  string
}

// GNS3: workspaces en Docker
type GNS3 struct {
	Volumes    string // carpeta del HOST con gns3_images, gns3_projects y ansible
	Imagen     string // imagen Docker del workspace
	PuertoBase int    // primer puerto interno; cada workspace usa un bloque de 200
}

type Config struct {
	Puerto         string
	Entorno        string
	WebDir         string
	JWTSecret      string
	DuracionSesion time.Duration

	AdminClave    string
	AdminPassword string

	DB    BaseDatos
	Redis Redis
	GNS3  GNS3
}

func env(clave, porDefecto string) string {
	if v := strings.TrimSpace(os.Getenv(clave)); v != "" {
		return v
	}
	return porDefecto
}

// Cargar devuelve un puntero: toda la app usa la MISMA configuración, sin copias.
func Cargar() (*Config, error) {
	puertoDB, err := strconv.Atoi(env("DB_PORT", "3050"))
	if err != nil {
		return nil, fmt.Errorf("DB_PORT inválido: %w", err)
	}

	puertoGNS3, err := strconv.Atoi(env("GNS3_BASE_PORT", "30000"))
	if err != nil {
		return nil, fmt.Errorf("GNS3_BASE_PORT inválido: %w", err)
	}

	cfg := &Config{
		Puerto:         env("PORT", "8080"),
		Entorno:        env("ENV", "desconocido"),
		WebDir:         env("WEB_DIR", "/app/web"),
		JWTSecret:      env("JWT_SECRET", ""),
		DuracionSesion: 8 * time.Hour,

		AdminClave:    strings.ToLower(env("ADMIN_CLAVE", "admin")),
		AdminPassword: env("ADMIN_PASSWORD", ""),

		DB: BaseDatos{
			Usuario:  env("DB_USER", "SYSDBA"),
			Password: env("DB_PASSWORD", ""),
			Host:     env("DB_HOST", "127.0.0.1"),
			Puerto:   puertoDB,
			Ruta:     env("DB_PATH", "/var/lib/firebird/data/"+env("DB_NAME", "raccoon.fdb")),
		},
		Redis: Redis{
			Direccion: env("REDIS_ADDR", "127.0.0.1:6379"),
			Password:  env("REDIS_PASSWORD", ""),
		},
		GNS3: GNS3{
			Volumes:    env("GNS3_VOLUMES", ""),
			Imagen:     env("GNS3_WORKSPACE_IMAGE", "gns3-workspace:2.2.59"),
			PuertoBase: puertoGNS3,
		},
	}

	if err := cfg.validar(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validar usa receptor puntero: revisa el objeto original, no una copia
func (c *Config) validar() error {
	faltan := []string{}
	if c.DB.Password == "" {
		faltan = append(faltan, "DB_PASSWORD")
	}
	if c.Redis.Password == "" {
		faltan = append(faltan, "REDIS_PASSWORD")
	}
	if len(c.JWTSecret) < 32 {
		faltan = append(faltan, "JWT_SECRET (mínimo 32 caracteres)")
	}
	if len(c.AdminPassword) < 8 {
		faltan = append(faltan, "ADMIN_PASSWORD (mínimo 8 caracteres)")
	}
	if c.GNS3.Volumes == "" {
		faltan = append(faltan, "GNS3_VOLUMES")
	}
	if len(faltan) > 0 {
		return fmt.Errorf("faltan variables en el .env: %s", strings.Join(faltan, ", "))
	}
	return nil
}
