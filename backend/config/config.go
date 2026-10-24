package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Env struct {
	Port        string
	AppID       string
	DNS         string
	RedisHost   string
	RedisPort   int
	RedisPasswd string
	AdminUser   string
	AdminPasswd string

	// Laboratorios GNS3 (corren en Docker)
	DockerSocket string // socket de Docker del host
	GNS3Volumes  string // carpeta con gns3_images, gns3_projects y ansible
	GNS3Image    string // imagen del laboratorio
	GNS3Host     string // dónde la API alcanza los puertos publicados (127.0.0.1)
	GNS3Port     int    // primer puerto del rango de laboratorios
	LabPuertos   int    // puertos por laboratorio (1 de GNS3 + consolas)
	LabsMax      int    // laboratorios al mismo tiempo
}

// LoadConfig regresa un puntero: toda la app comparte la MISMA configuración
func LoadConfig() *Env {

	// En desarrollo lee el archivo .env; en el contenedor Podman ya inyecta las variables
	if err := godotenv.Load(); err != nil {
		log.Println("Sin archivo .env, se usan las variables del entorno")
	}

	env := &Env{
		Port:        os.Getenv("APP_PORT"),
		AppID:       os.Getenv("APP_ID"),
		DNS:         os.Getenv("DB_DSN"),
		RedisHost:   os.Getenv("REDIS_HOST"),
		RedisPort:   entero("REDIS_PORT", 6379),
		RedisPasswd: os.Getenv("REDIS_PASSWORD"),
		AdminUser:   os.Getenv("ADMIN_USER"),
		AdminPasswd: os.Getenv("ADMIN_PASSWORD"),

		DockerSocket: texto("DOCKER_SOCKET", "/var/run/docker.sock"),
		GNS3Volumes:  os.Getenv("GNS3_VOLUMES"),
		GNS3Image:    texto("GNS3_WORKSPACE_IMAGE", "gns3-workspace:2.2.59"),
		GNS3Host:     texto("GNS3_HOST", "127.0.0.1"),
		GNS3Port:     entero("GNS3_BASE_PORT", 30000),
		LabPuertos:   entero("LAB_PUERTOS", 40),
		LabsMax:      entero("LABS_MAX", 60),
	}

	validarVolumenes(env.GNS3Volumes)
	return env
}

// validarVolumenes: Docker monta gns3_images, gns3_projects y ansible desde esta carpeta
// del host; si la ruta está mal los laboratorios no funcionan, así que la API no arranca
func validarVolumenes(ruta string) {
	if ruta == "" {
		log.Fatal("Falta GNS3_VOLUMES en el .env (carpeta con gns3_images, gns3_projects y ansible)")
	}
	if !filepath.IsAbs(ruta) {
		log.Fatalf("GNS3_VOLUMES debe ser una ruta absoluta (empieza con /): %q", ruta)
	}
	if info, err := os.Stat(ruta); err != nil || !info.IsDir() {
		log.Fatalf("GNS3_VOLUMES no es una carpeta que exista: %q", ruta)
	}
}

// texto lee una variable o usa el valor por defecto
func texto(clave, porDefecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return porDefecto
}

// entero lee una variable numérica o usa el valor por defecto
func entero(clave string, porDefecto int) int {
	v, err := strconv.Atoi(os.Getenv(clave))
	if err != nil {
		return porDefecto
	}
	return v
}
