// Package api: handlers HTTP. Todos son métodos de *App (sin variables globales).
package api

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"time"

	"raccoon_lab/internal/config"
	"raccoon_lab/internal/docker"
	"raccoon_lab/internal/importar"
	"raccoon_lab/internal/labs"
	"raccoon_lab/internal/repositorio"
	"raccoon_lab/internal/seguridad"
	"raccoon_lab/internal/sesiones"

	"github.com/redis/go-redis/v9"
)

// App agrupa TODAS las dependencias. Se crea una sola vez y se comparte por puntero.
type App struct {
	Cfg        *config.Config
	DB         *sql.DB
	Redis      *redis.Client
	Sesiones   *sesiones.Store
	Usuarios   *repositorio.UsuarioRepo
	Grupos     *repositorio.GrupoRepo
	Limitador  *seguridad.Limitador
	Importador *importar.Importador
	Docker     *docker.Cliente
	Labs       *labs.Servicio
	Version    string
	Inicio     time.Time
}

func NuevaApp(cfg *config.Config, db *sql.DB, rdb *redis.Client, version string) *App {
	ses := &sesiones.Store{R: rdb, Duracion: cfg.DuracionSesion}

	imp := importar.Nuevo(db, ses.EliminarTodas, os.TempDir())
	imp.Protegida = cfg.AdminClave

	dk := docker.Nuevo(docker.SocketPorDefecto)
	laboratorios := labs.Nuevo(cfg, dk, &repositorio.WorkspaceRepo{DB: db})
	// Al arrancar: reabrir los proxies de los laboratorios que siguen vivos
	go laboratorios.Restaurar(context.Background())

	return &App{
		Cfg:        cfg,
		DB:         db,
		Redis:      rdb,
		Sesiones:   ses,
		Usuarios:   &repositorio.UsuarioRepo{DB: db, AdminClave: cfg.AdminClave},
		Grupos:     &repositorio.GrupoRepo{DB: db},
		Limitador:  seguridad.NuevoLimitador(5, 15*time.Minute),
		Importador: imp,
		Docker:     dk,
		Labs:       laboratorios,
		Version:    version,
		Inicio:     time.Now(),
	}
}

// Rutas registra todos los endpoints
func (a *App) Rutas() http.Handler {
	mux := http.NewServeMux()

	// Públicas
	mux.HandleFunc("GET /api/salud", a.Salud)
	mux.HandleFunc("POST /api/auth/login", a.Login)
	mux.HandleFunc("POST /api/auth/refresh", a.Refresh)
	mux.HandleFunc("POST /api/auth/logout", a.Logout)

	// Con sesión (cualquier rol)
	mux.Handle("POST /api/auth/cambiar-rol", a.ConSesion(a.CambiarRol))
	mux.Handle("GET /api/usuario/perfil", a.ConSesion(a.ObtenerPerfil))
	mux.Handle("PUT /api/usuario/perfil/password", a.ConSesion(a.CambiarMiPassword))

	// Solo ADMIN: importación masiva de usuarios
	mux.Handle("POST /api/admin/importar/{tipo}", a.ConRol(a.SubirImportacion, "ADMIN"))
	mux.Handle("GET /api/admin/importar/{id}", a.ConRol(a.EstadoImportacion, "ADMIN"))
	mux.Handle("GET /api/admin/importar/plantilla/{tipo}", a.ConRol(a.PlantillaImportacion, "ADMIN"))

	// Solo ADMIN: usuarios
	mux.Handle("GET /api/admin/usuarios", a.ConRol(a.ListarUsuarios, "ADMIN"))
	mux.Handle("GET /api/admin/usuarios/resumen", a.ConRol(a.ResumenUsuarios, "ADMIN"))
	mux.Handle("PUT /api/admin/usuarios/editar", a.ConRol(a.EditarUsuario, "ADMIN"))
	mux.Handle("PUT /api/admin/usuarios/password", a.ConRol(a.ResetPassword, "ADMIN"))
	mux.Handle("PUT /api/admin/usuarios/datos", a.ConRol(a.EditarDatos, "ADMIN"))
	mux.Handle("DELETE /api/admin/usuarios/{matricula}", a.ConRol(a.EliminarUsuario, "ADMIN"))

	// Grupos: ADMIN ve todos, PROFESOR solo los suyos
	mux.Handle("GET /api/grupos", a.ConRol(a.ListarGrupos, "ADMIN", "PROFESOR"))
	mux.Handle("GET /api/grupos/{codigo}/alumnos", a.ConRol(a.AlumnosDeGrupo, "ADMIN", "PROFESOR"))

	// Solo ADMIN: administrar grupos
	mux.Handle("GET /api/admin/maestros", a.ConRol(a.MaestrosDisponibles, "ADMIN"))
	mux.Handle("POST /api/admin/grupos", a.ConRol(a.CrearGrupo, "ADMIN"))
	mux.Handle("DELETE /api/admin/grupos/{codigo}", a.ConRol(a.EliminarGrupo, "ADMIN"))
	mux.Handle("PUT /api/admin/grupos/{codigo}/docentes", a.ConRol(a.AsignarDocentes, "ADMIN"))

	// Laboratorios GNS3 (cualquier rol)
	mux.Handle("GET /api/labs/estado", a.ConSesion(a.EstadoLab))
	mux.Handle("POST /api/labs/crear", a.ConSesion(a.CrearLab))
	mux.Handle("POST /api/labs/limpiar", a.ConSesion(a.LimpiarLab))

	// API que aún no existe -> 404 JSON
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		responderError(w, http.StatusNotFound, "Ruta de API no encontrada")
	})

	// Frontend (Vue)
	mux.Handle("/", frontend(a.Cfg.WebDir))
	return mux
}
