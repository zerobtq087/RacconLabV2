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

	// El servicio no toca Redis: el middleware le presta esta función para cerrar sesiones
	cerrarSesiones := func(ctx context.Context, userID string) { _ = s.CloseAll(ctx, userID, "") }

	// Usuarios (solo admin): tabla, agregar, editar y borrar
	usuarios := api.Group("/usuarios", s.Validate, admin)
	usuarioRepo := repository.NewUsuarioRepository(db)
	usuarioHandler := handler.NewUsuarioHandler(s, usuarioRepo,
		service.NewUsuarios(usuarioRepo, cerrarSesiones, env.AdminUser), env.AdminUser)
	usuarios.Get("/", usuarioHandler.GetPage)
	usuarios.Get("/tipos", usuarioHandler.Tipos)
	usuarios.Post("/", usuarioHandler.Create)
	usuarios.Put("/:id", usuarioHandler.Update)
	usuarios.Delete("/:id", usuarioHandler.Delete)
	api.Get("/maestros", s.Validate, admin, usuarioHandler.Maestros)

	// Importar usuarios desde CSV/Excel (solo admin)
	importacionRepo := repository.NewImportacionRepository(db)
	importador := service.NewImportador(importacionRepo, cerrarSesiones, env.AdminUser)
	importarHandler := handler.NewImportarHandler(importacionRepo, importador)
	api.Post("/importar", s.Validate, admin, importarHandler.Upload)
	api.Get("/importar/plantilla", s.Validate, admin, importarHandler.Plantilla) // antes de /:id
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
	plantillas.Post("/redes", plantillaHandler.Redes)

	// Prácticas con calificación: el profesor (o admin) las arma desde un proyecto de su lab
	practicaRepo := repository.NewPracticaRepository(db)
	practicasService := service.NewPracticas(practicaRepo, laboratorioRepo, docker, env)
	practicaHandler := handler.NewPracticaHandler(practicasService)
	practicas := api.Group("/practicas", s.Validate, adminProfesor)
	practicas.Get("/", practicaHandler.Listar)
	practicas.Get("/proyectos", practicaHandler.Proyectos) // antes de /:id
	practicas.Post("/", practicaHandler.Crear)
	practicas.Get("/:id", practicaHandler.Detalle)
	practicas.Put("/:id", practicaHandler.Editar)
	practicas.Delete("/:id", practicaHandler.Borrar)
	practicas.Put("/:id/criterios", practicaHandler.Criterios)
	practicas.Post("/:id/activar", practicaHandler.Activar)
	practicas.Post("/:id/cerrar", practicaHandler.Cerrar)
	practicas.Post("/:id/prueba", practicaHandler.Probar)
	practicas.Get("/:id/prueba", practicaHandler.Prueba)
	practicas.Get("/:id/entregas", practicaHandler.Entregas)
	practicas.Get("/:id/excel", practicaHandler.Excel)
	practicas.Get("/:id/entregas/:alumno", practicaHandler.Entrega)
	practicas.Get("/:id/entregas/:alumno/proyecto", practicaHandler.ProyectoEntrega)
	practicas.Post("/:id/entregas/:alumno/abrir", practicaHandler.AbrirEntrega)

	// El alumno abre la práctica en su lab y la entrega una sola vez
	misPracticas := api.Group("/mis-practicas", s.Validate, s.RequireRol("ALUMNO"))
	misPracticas.Get("/", practicaHandler.MisPracticas)
	misPracticas.Post("/:id/abrir", practicaHandler.Abrir)
	misPracticas.Post("/:id/entregar", practicaHandler.Entregar)

	// Supervisión: el docente/admin ve los labs de sus alumnos
	accesoRepo := repository.NewAccesoRepository(db)
	supervisionHandler := handler.NewSupervisionHandler(
		service.NewSupervision(practicaRepo, accesoRepo, laboratorioRepo, docker, env))
	supervision := api.Group("/supervision", s.Validate, adminProfesor)
	supervision.Get("/bitacora", supervisionHandler.Bitacora)
	supervision.Get("/grupos/:grupo", supervisionHandler.Alumnos)
	supervision.Get("/labs/:lab/proyectos", supervisionHandler.Proyectos)
	supervision.Get("/labs/:lab/proyectos/:proyecto/nodos", supervisionHandler.Nodos)
	supervision.Get("/labs/:lab/proyectos/:proyecto/descargar", supervisionHandler.Descargar)
	supervision.Post("/labs/:lab/proyectos/:proyecto/copiar", supervisionHandler.Copiar)
	supervision.Post("/labs/:lab/consolas", supervisionHandler.Consolas)
	supervision.Delete("/labs/:lab/consolas", supervisionHandler.CerrarConsolas)

	// Interno: el agente del firewall (raccoon-accesos) pide las IPs con acceso a
	// cada lab. Sin sesión; lo protege la clave ACCESOS_TOKEN del .env.
	api.Get("/interno/accesos", handler.NewInternoHandler(service.NewAccesos(accesoRepo, env), env).Accesos)

	// Inicio: tablero según el rol con el que se entró (un solo endpoint)
	inicioHandler := handler.NewInicioHandler(service.NewInicio(repository.NewInicioRepository(db),
		laboratorioRepo, practicasService, practicaRepo, accesoRepo, docker, env))
	api.Get("/inicio", s.Validate, inicioHandler.Tablero)

	// Monitor de laboratorios (solo admin)
	api.Get("/monitor", s.Validate, admin, laboratorioHandler.Monitor)
	api.Delete("/monitor/:id", s.Validate, admin, laboratorioHandler.Close)
}
