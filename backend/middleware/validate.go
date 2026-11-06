package middleware

import (
	"context"
	"slices"
	"strconv"
	"time"

	"raccoon-lab/backend/repository"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/storage/redis/v3"
)

// Session es lo ÚNICO que toca la sesión de Redis.
// Los handlers leen los datos con c.Locals(...)
// accesos guarda en Firebird la IP de cada sesión: el firewall abre los puertos
// del lab solo a las IPs con sesión (ver repository/acceso_repo.go).
type Session struct {
	store   *session.Store
	storage *redis.Storage
	accesos *repository.AccesoRepository
}

func NewSession(store *session.Store, storage *redis.Storage, accesos *repository.AccesoRepository) *Session {
	return &Session{store: store, storage: storage, accesos: accesos}
}

// cadaCuanto: cada cuánto se renueva la sesión con IP en Firebird si no cambia la IP
const cadaCuanto = 2 * time.Minute

// recordarIP guarda la IP de la sesión en Firebird si cambió o ya pasó "cadaCuanto"
// (así no se escribe en la base en cada petición)
func (s *Session) recordarIP(c fiber.Ctx, sess *session.Session, userID string) {
	ip := c.IP() // la del navegador (Nginx la manda en X-Real-IP)
	if ip == "" {
		// sin Nginx (desarrollo) no viene X-Real-IP: se usa la de la conexión
		ip = c.RequestCtx().RemoteIP().String()
	}
	visto, _ := strconv.ParseInt(stringDe(sess.Get("visto")), 10, 64)
	if stringDe(sess.Get("ip")) == ip && time.Since(time.Unix(visto, 0)) < cadaCuanto {
		return
	}
	s.accesos.GuardarSesion(sess.ID(), userID, ip)
	sess.Set("ip", ip)
	sess.Set("visto", strconv.FormatInt(time.Now().Unix(), 10))
}

func stringDe(v any) string {
	s, _ := v.(string)
	return s
}

// clave en Redis con las sesiones abiertas de un usuario
func indice(userID string) string {
	return "sesiones:" + userID
}

// Validate verifica que exista una sesión válida
func (s *Session) Validate(c fiber.Ctx) error {
	sess, err := s.store.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error de sesión")
	}
	defer sess.Release()

	// Verificar si existe userID en la sesión
	userID := sess.Get("userID")
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).SendString("No autorizado")
	}

	// La IP de esta sesión abre los puertos de su lab en el firewall
	s.recordarIP(c, sess, stringDe(userID))

	// Pasa datos a los siguientes handlers mediante Locals
	c.Locals("sessionID", sess.ID())
	c.Locals("userID", userID)
	c.Locals("matricula", sess.Get("matricula"))
	c.Locals("nombre", sess.Get("nombre"))
	c.Locals("rol", sess.Get("rol"))

	// Renueva la cookie (la sesión dura mientras haya actividad)
	if err := sess.Save(); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error de sesión")
	}

	return c.Next()
}

// RequireRol deja pasar solo si entró con alguno de esos roles.
// Se usa DESPUÉS de Validate: api.Get("/x", s.Validate, s.RequireRol("ADMIN"), ...)
func (s *Session) RequireRol(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		rol, _ := c.Locals("rol").(string)
		if !slices.Contains(roles, rol) {
			return c.Status(fiber.StatusForbidden).SendString("No tienes permiso para esto")
		}
		return c.Next()
	}
}

// Create abre la sesión del usuario (la llama el login)
func (s *Session) Create(c fiber.Ctx, userID, matricula, nombre, rol string) error {
	sess, err := s.store.Get(c)
	if err != nil {
		return err
	}
	defer sess.Release()

	// Nuevo ID de sesión en cada login
	// Si en esta computadora había otra sesión, su IP deja de abrir aquel lab
	anterior := sess.ID()
	if err := sess.Regenerate(); err != nil {
		return err
	}
	s.accesos.QuitarSesion(anterior)
	sess.Set("visto", "0") // fuerza a guardar la IP con el ID nuevo
	sess.Set("userID", userID)
	sess.Set("matricula", matricula)
	sess.Set("nombre", nombre)
	sess.Set("rol", rol)
	s.recordarIP(c, sess, userID)

	// Guardar la sesión (esto envía la cookie al cliente)
	if err := sess.Save(); err != nil {
		return err
	}

	// Recordar qué sesiones tiene abiertas el usuario
	return s.storage.Conn().SAdd(c.Context(), indice(userID), sess.ID()).Err()
}

// Destroy cierra la sesión actual (logout)
func (s *Session) Destroy(c fiber.Ctx) error {
	sess, err := s.store.Get(c)
	if err != nil {
		return err
	}
	defer sess.Release()

	if userID, ok := sess.Get("userID").(string); ok {
		s.storage.Conn().SRem(c.Context(), indice(userID), sess.ID())
	}
	s.accesos.QuitarSesion(sess.ID()) // se cierran los puertos de su lab para esa IP
	return sess.Destroy()
}

// CloseAll cierra todas las sesiones de un usuario, menos "excepto"
// (al cambiar contraseña, al editarlo o al borrarlo)
func (s *Session) CloseAll(ctx context.Context, userID, excepto string) error {
	conn := s.storage.Conn()

	ids, err := conn.SMembers(ctx, indice(userID)).Result()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == excepto {
			continue
		}
		_ = s.store.Delete(ctx, id)
		conn.SRem(ctx, indice(userID), id)
	}
	s.accesos.QuitarSesiones(userID, excepto)
	return nil
}
