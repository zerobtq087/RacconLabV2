package config

import (
	"log"
	"os"
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
	GNS3Volumes string
	GNS3Image   string
	GNS3Port    int
}

// LoadConfig regresa un puntero: toda la app comparte la MISMA configuración
func LoadConfig() *Env {

	// En desarrollo lee el archivo .env; en el contenedor Podman ya inyecta las variables
	if err := godotenv.Load(); err != nil {
		log.Println("Sin archivo .env, se usan las variables del entorno")
	}

	redisPort, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		redisPort = 6379
	}

	gns3Port, err := strconv.Atoi(os.Getenv("GNS3_BASE_PORT"))
	if err != nil {
		gns3Port = 30000
	}

	return &Env{
		Port:        os.Getenv("APP_PORT"),
		AppID:       os.Getenv("APP_ID"),
		DNS:         os.Getenv("DB_DSN"),
		RedisHost:   os.Getenv("REDIS_HOST"),
		RedisPort:   redisPort,
		RedisPasswd: os.Getenv("REDIS_PASSWORD"),
		AdminUser:   os.Getenv("ADMIN_USER"),
		AdminPasswd: os.Getenv("ADMIN_PASSWORD"),
		GNS3Volumes: os.Getenv("GNS3_VOLUMES"),
		GNS3Image:   os.Getenv("GNS3_WORKSPACE_IMAGE"),
		GNS3Port:    gns3Port,
	}
}
