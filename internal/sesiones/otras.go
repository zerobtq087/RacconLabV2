package sesiones

import (
	"context"

	"raccoon_lab/internal/modelos"
)

// EliminarOtras cierra todas las sesiones del usuario MENOS la actual
// (al cambiar su propia contraseña sigue dentro en este navegador)
func (st *Store) EliminarOtras(ctx context.Context, actual *modelos.Sesion) error {
	ids, err := st.R.SMembers(ctx, prefijoUsuario+actual.Matricula).Result()
	if err != nil {
		return err
	}
	pipe := st.R.TxPipeline()
	for _, id := range ids {
		if id != actual.ID {
			pipe.Del(ctx, prefijoSesion+id)
			pipe.SRem(ctx, prefijoUsuario+actual.Matricula, id)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}
