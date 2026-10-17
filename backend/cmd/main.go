package main

import (
	"log"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/database"
	"raccoon-lab/backend/middleware"
	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/router"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/storage/redis/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load config Env variables
	env := config.LoadConfig()

	// DB Connection
	db := database.ConnectDB(env.DNS)

	// Admin general (si no existe)
	createAdmin(env, repository.NewAuthRepository(db))

	// Redis for session store (Redis SOLO guarda sesiones)
	storage := redis.New(redis.Config{
		Host:     env.RedisHost,
		Port:     env.RedisPort,
		Password: env.RedisPasswd,
		Database: 0,
		Reset:    false,
	})

	store := session.NewStore(session.Config{
		Storage:         storage,
		IdleTimeout:     8 * time.Hour,
		AbsoluteTimeout: 12 * time.Hour,
		Extractor:       extractors.FromCookie("rl_sesion"),
		CookieHTTPOnly:  true,
		CookieSameSite:  "Lax",
	})
	sess := middleware.NewSession(store, storage)

	// Fiber Web server (solo API: el frontend lo sirve su propio contenedor)
	app := fiber.New(fiber.Config{
		AppName:   "Raccoon Lab " + env.AppID,
		BodyLimit: 20 * 1024 * 1024, // archivos CSV/Excel de hasta 20 MB
	})
	app.Use(helmet.New())

	router.SetupRoutes(app, db, sess)

	if err := app.Listen(env.Port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func createAdmin(env *config.Env, repo *repository.AuthRepository) {
	if env.AdminUser == "" || len(env.AdminPasswd) < 8 {
		log.Fatal("ADMIN_USER y ADMIN_PASSWORD (mínimo 8 caracteres) son obligatorios en el .env")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(env.AdminPasswd), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	if err := repo.CreateAdmin(uuid.NewString(), env.AdminUser, string(hash)); err != nil {
		log.Fatal("No se pudo crear el admin general: ", err)
	}
}
