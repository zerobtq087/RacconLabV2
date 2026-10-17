package middleware

import (
	"context"
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/storage/redis/v3"
)

// Session es lo ÚNICO que toca la sesión de Redis.
// Los handlers leen los datos con c.Locals(...)
type Session struct {
	store   *session.Store
	storage *redis.Storage
}

func NewSession(store *session.Store, storage *redis.Storage) *Session {
	return &Session{store: store, storage: storage}
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
	if err := sess.Regenerate(); err != nil {
		return err
	}
	sess.Set("userID", userID)
	sess.Set("matricula", matricula)
	sess.Set("nombre", nombre)
	sess.Set("rol", rol)

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
	return nil
}
