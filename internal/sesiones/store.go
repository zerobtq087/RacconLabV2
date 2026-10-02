package sesiones

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"raccoon_lab/internal/modelos"

	"github.com/redis/go-redis/v9"
)

// Claves en Redis:
//
//	sesion:<id>             -> JSON de la sesión (expira sola)
//	sesiones:<matricula>    -> conjunto con los <id> activos del usuario
const (
	prefijoSesion  = "sesion:"
	prefijoUsuario = "sesiones:"
)

var ErrNoExiste = errors.New("sesión no encontrada o expirada")

// Store administra las sesiones. Comparte el *redis.Client de toda la app.
type Store struct {
	R        *redis.Client
	Duracion time.Duration
}

func nuevoID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Crear guarda la sesión y le asigna un ID aleatorio (se escribe en s.ID)
func (st *Store) Crear(ctx context.Context, s *modelos.Sesion) error {
	id, err := nuevoID()
	if err != nil {
		return fmt.Errorf("generando id de sesión: %w", err)
	}
	s.ID = id
	s.CreadaEn = time.Now().Unix()
	return st.guardar(ctx, s)
}

func (st *Store) guardar(ctx context.Context, s *modelos.Sesion) error {
	datos, err := json.Marshal(s)
	if err != nil {
		return err
	}
	pipe := st.R.TxPipeline()
	pipe.Set(ctx, prefijoSesion+s.ID, datos, st.Duracion)
	pipe.SAdd(ctx, prefijoUsuario+s.Matricula, s.ID)
	pipe.Expire(ctx, prefijoUsuario+s.Matricula, st.Duracion)
	_, err = pipe.Exec(ctx)
	return err
}

// Obtener lee la sesión y renueva su expiración (sesión deslizante)
func (st *Store) Obtener(ctx context.Context, id string) (*modelos.Sesion, error) {
	if id == "" {
		return nil, ErrNoExiste
	}
	datos, err := st.R.Get(ctx, prefijoSesion+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoExiste
	}
	if err != nil {
		return nil, err
	}

	s := &modelos.Sesion{}
	if err := json.Unmarshal(datos, s); err != nil {
		return nil, err
	}
	s.ID = id
	st.R.Expire(ctx, prefijoSesion+id, st.Duracion)
	return s, nil
}

// CambiarRol actualiza el rol activo de una sesión existente
func (st *Store) CambiarRol(ctx context.Context, s *modelos.Sesion, rol string) error {
	s.RolActivo = rol
	return st.guardar(ctx, s)
}

// Eliminar cierra una sesión (logout)
func (st *Store) Eliminar(ctx context.Context, s *modelos.Sesion) error {
	pipe := st.R.TxPipeline()
	pipe.Del(ctx, prefijoSesion+s.ID)
	pipe.SRem(ctx, prefijoUsuario+s.Matricula, s.ID)
	_, err := pipe.Exec(ctx)
	return err
}

// EliminarTodas cierra TODAS las sesiones de un usuario
// (cuando el admin le cambia la contraseña, los roles o lo desactiva)
func (st *Store) EliminarTodas(ctx context.Context, matricula string) error {
	ids, err := st.R.SMembers(ctx, prefijoUsuario+matricula).Result()
	if err != nil {
		return err
	}
	pipe := st.R.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, prefijoSesion+id)
	}
	pipe.Del(ctx, prefijoUsuario+matricula)
	_, err = pipe.Exec(ctx)
	return err
}
