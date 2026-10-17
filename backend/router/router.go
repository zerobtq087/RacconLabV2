package router

import (
	"raccoon-lab/backend/handler"
	"raccoon-lab/backend/middleware"
	"raccoon-lab/backend/repository"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"gorm.io/gorm"
)

func SetupRoutes(app *fiber.App, db *gorm.DB, s *middleware.Session) {

	app.Use(logger.New())

	api := app.Group("/api")

	// Auth
	authHandler := handler.NewAuthHandler(s, repository.NewAuthRepository(db))
	api.Post("/login", authHandler.Login)
	api.Post("/logout", authHandler.Logout)
	api.Post("/update-passwd", s.Validate, authHandler.UpdatePasswd)
	api.Get("/current-user-acc", s.Validate, authHandler.CurrentUserAcc)
}
