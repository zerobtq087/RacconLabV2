package router

import (
	"context"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/handler"
	"raccoon-lab/backend/middleware"
	"raccoon-lab/backend/repository"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"gorm.io/gorm"
)

func SetupRoutes(app *fiber.App, env *config.Env, db *gorm.DB, s *middleware.Session) {

	app.Use(logger.New())

	api := app.Group("/api")

	// Quién puede entrar a cada grupo de rutas
	admin := s.RequireRol("ADMIN")
	adminProfesor := s.RequireRol("ADMIN", "PROFESOR")

	// Auth
	authHandler := handler.NewAuthHandler(s, repository.NewAuthRepository(db))
	api.Post("/login", authHandler.Login)
	api.Post("/logout", authHandler.Logout)
	api.Post("/update-passwd", s.Validate, authHandler.UpdatePasswd)
	api.Get("/current-user-acc", s.Validate, authHandler.CurrentUserAcc)

	// Usuarios (solo admin)
	usuarios := api.Group("/usuarios", s.Validate, admin)
	usuarioHandler := handler.NewUsuarioHandler(s, repository.NewUsuarioRepository(db), env.AdminUser)
	usuarios.Get("/", usuarioHandler.GetPage)
	usuarios.Put("/:id", usuarioHandler.Update)
	usuarios.Put("/:id/password", usuarioHandler.UpdatePasswd)
	usuarios.Delete("/:id", usuarioHandler.Delete)
	api.Get("/maestros", s.Validate, admin, usuarioHandler.Maestros)

	// Importar usuarios desde CSV/Excel (solo admin)
	importacionRepo := repository.NewImportacionRepository(db)
	cerrarSesiones := func(ctx context.Context, userID string) { _ = s.CloseAll(ctx, userID, "") }
	importador := service.NewImportador(importacionRepo, cerrarSesiones, env.AdminUser)
	importarHandler := handler.NewImportarHandler(importacionRepo, importador)
	api.Post("/importar", s.Validate, admin, importarHandler.Upload)
	api.Get("/importar/:id", s.Validate, admin, importarHandler.Status)

	// Grupos (admin todo; maestro solo ve los suyos)
	grupos := api.Group("/grupos", s.Validate, adminProfesor)
	grupoHandler := handler.NewGrupoHandler(repository.NewGrupoRepository(db))
	grupos.Get("/", grupoHandler.GetAll)
	grupos.Get("/:id/alumnos", grupoHandler.Alumnos)
	grupos.Post("/", admin, grupoHandler.Create)
	grupos.Put("/:id", admin, grupoHandler.Update)
	grupos.Delete("/:id", admin, grupoHandler.Delete)
	grupos.Put("/:id/docentes", admin, grupoHandler.SetDocentes)
	api.Get("/carreras", s.Validate, admin, grupoHandler.Carreras)

	// Laboratorios GNS3 (alumno y maestro)
	docker := service.NewDocker(env.DockerSocket)
	laboratorioRepo := repository.NewLaboratorioRepository(db)
	laboratorios := service.NewLaboratorios(laboratorioRepo, docker, env)
	laboratorioHandler := handler.NewLaboratorioHandler(laboratorioRepo, laboratorios)
	labs := api.Group("/labs", s.Validate, s.RequireRol("ALUMNO", "PROFESOR"))
	labs.Get("/", laboratorioHandler.Status)
	labs.Post("/", laboratorioHandler.Create)
	labs.Delete("/", laboratorioHandler.Leave)
	labs.Post("/unirse", laboratorioHandler.Join)
	labs.Get("/companeros", laboratorioHandler.Companeros)
	labs.Get("/invitaciones", laboratorioHandler.Invitations)
	labs.Post("/invitaciones", laboratorioHandler.Invite)
	labs.Put("/invitaciones/:id", laboratorioHandler.Answer)

	// Plantillas: el profesor despliega topologías en SU laboratorio (en segundo plano)
	despliegues := service.NewDespliegues(repository.NewDespliegueRepository(db), laboratorioRepo, docker, env)
	plantillaHandler := handler.NewPlantillaHandler(despliegues)
	plantillas := api.Group("/plantillas", s.Validate, s.RequireRol("PROFESOR"))
	plantillas.Post("/despliegue", plantillaHandler.Desplegar)
	plantillas.Get("/despliegue", plantillaHandler.Estado)

	// Monitor de laboratorios (solo admin)
	api.Get("/monitor", s.Validate, admin, laboratorioHandler.Monitor)
	api.Delete("/monitor/:id", s.Validate, admin, laboratorioHandler.Close)
}
