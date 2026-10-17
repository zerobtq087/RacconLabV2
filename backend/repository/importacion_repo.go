package repository

import (
	"errors"
	"strings"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrImportacionOcupada = errors.New("ya hay una importación en curso, espera a que termine")

const maxErroresGuardados = 200

type ImportacionRepository struct {
	db *gorm.DB
}

func NewImportacionRepository(db *gorm.DB) *ImportacionRepository {
	return &ImportacionRepository{db: db}
}

// Reserve registra una importación nueva. Solo puede haber UNA en curso,
// aunque haya 2 réplicas de la API (la otra la ve en la base).
func (r *ImportacionRepository) Reserve(tipo, archivo, idUsuario string) (string, error) {
	// Si una réplica murió a medio importar, su latido se detiene: se da por fallida
	r.db.Exec(`
		UPDATE IMPORTACIONES SET ESTADO = 'ERROR', MENSAJE = 'Se interrumpió (el servidor se reinició)',
		       FIN = CURRENT_TIMESTAMP
		WHERE ESTADO = 'PROCESANDO' AND ACTUALIZADO < DATEADD(-2 MINUTE TO CURRENT_TIMESTAMP)`)

	var enCurso int
	r.db.Raw(`SELECT COUNT(*) FROM IMPORTACIONES WHERE ESTADO = 'PROCESANDO'`).Scan(&enCurso)
	if enCurso > 0 {
		return "", ErrImportacionOcupada
	}

	id := uuid.NewString()
	err := r.db.Exec(`
		INSERT INTO IMPORTACIONES (ID, TIPO, ARCHIVO, ID_USUARIO)
		VALUES (CHAR_TO_UUID(?), ?, ?, CHAR_TO_UUID(?))`, id, tipo, archivo, idUsuario).Error
	return id, err
}

func (r *ImportacionRepository) GetByID(id string) *domain.Importacion {
	var imp domain.Importacion

	r.db.Raw(`
		SELECT UUID_TO_CHAR(ID) ID, TIPO, ARCHIVO, ESTADO, COALESCE(MENSAJE, '') MENSAJE,
		       PORCENTAJE, LEIDAS, INSERTADOS, ACTUALIZADOS, CON_ERROR
		FROM IMPORTACIONES
		WHERE ID = CHAR_TO_UUID(?)`, id).Scan(&imp)

	if imp.ID == "" {
		return nil
	}

	imp.Errores = []domain.ErrorFila{}
	r.db.Raw(`
		SELECT FILA, MENSAJE FROM IMPORTACION_ERRORES
		WHERE ID_IMPORTACION = CHAR_TO_UUID(?)
		ORDER BY FILA`, id).Scan(&imp.Errores)

	return &imp
}

// SaveProgress guarda contadores y renueva el latido
func (r *ImportacionRepository) SaveProgress(imp *domain.Importacion) {
	r.db.Exec(`
		UPDATE IMPORTACIONES SET
			PORCENTAJE = ?, LEIDAS = ?, INSERTADOS = ?, ACTUALIZADOS = ?, CON_ERROR = ?,
			ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`,
		imp.Porcentaje, imp.Leidas, imp.Insertados, imp.Actualizados, imp.ConError, imp.ID)
}

// SaveErrors guarda las filas con error (máximo 200 por importación)
func (r *ImportacionRepository) SaveErrors(id string, errores []domain.ErrorFila, yaGuardados int) {
	for _, e := range errores {
		if yaGuardados >= maxErroresGuardados {
			return
		}
		msg := e.Mensaje
		if len(msg) > 300 {
			msg = msg[:300]
		}
		r.db.Exec(`
			UPDATE OR INSERT INTO IMPORTACION_ERRORES (ID_IMPORTACION, FILA, MENSAJE)
			VALUES (CHAR_TO_UUID(?), ?, ?) MATCHING (ID_IMPORTACION, FILA)`, id, e.Fila, msg)
		yaGuardados++
	}
}

func (r *ImportacionRepository) Finish(imp *domain.Importacion) {
	msg := imp.Mensaje
	if len(msg) > 300 {
		msg = msg[:300]
	}
	r.SaveProgress(imp)
	r.db.Exec(`
		UPDATE IMPORTACIONES SET ESTADO = ?, MENSAJE = ?, FIN = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`, imp.Estado, msg, imp.ID)
}

// RolesDeTipo: los roles que recibe cada tipo (catálogo TIPO_ROLES)
func (r *ImportacionRepository) RolesDeTipo(tipo string) []string {
	roles := []string{}
	r.db.Raw(`SELECT ROL FROM TIPO_ROLES WHERE TIPO = ?`, tipo).Scan(&roles)
	for i := range roles {
		roles[i] = strings.TrimSpace(roles[i])
	}
	return roles
}

// ResultadoLote: lo que pasó al guardar un lote
type ResultadoLote struct {
	Nuevos       int
	Actualizados []string // IDs de usuarios que ya existían (se cierran sus sesiones)
	Errores      []domain.ErrorFila
}

// SaveBatch guarda un lote de filas en UNA transacción.
// Un error en una fila no tumba el lote: se anota y se sigue.
// grupos es un caché código -> ID que se comparte entre lotes (por apuntador).
func (r *ImportacionRepository) SaveBatch(tipo string, roles []string, lote []*domain.FilaImport,
	grupos map[string]string) (*ResultadoLote, error) {

	res := &ResultadoLote{}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, f := range lote {
			// Grupo (solo alumnos): se crea si no existe
			var idGrupo *string
			if f.Grupo != "" {
				id, err := grupoID(tx, f.Grupo, f.Carrera, grupos)
				if err != nil {
					res.Errores = append(res.Errores, domain.ErrorFila{Fila: f.Num, Mensaje: "no se pudo crear el grupo " + f.Grupo})
					continue
				}
				idGrupo = &id
			}

			var idUsuario string
			tx.Raw(`SELECT UUID_TO_CHAR(ID) FROM USUARIOS WHERE MATRICULA = ?`, f.Matricula).Scan(&idUsuario)

			if idUsuario == "" {
				idUsuario = uuid.NewString()
				if err := tx.Exec(`
					INSERT INTO USUARIOS (ID, MATRICULA, NOMBRE, "PASSWORD", TIPO, ID_GRUPO)
					VALUES (CHAR_TO_UUID(?), ?, ?, ?, ?, CHAR_TO_UUID(?))`,
					idUsuario, f.Matricula, f.Nombre, f.Hash, tipo, idGrupo).Error; err != nil {
					res.Errores = append(res.Errores, domain.ErrorFila{Fila: f.Num, Mensaje: "error al insertar"})
					continue
				}
				res.Nuevos++
			} else {
				if err := tx.Exec(`
					UPDATE USUARIOS SET NOMBRE = ?, "PASSWORD" = ?, TIPO = ?, ID_GRUPO = CHAR_TO_UUID(?)
					WHERE ID = CHAR_TO_UUID(?)`,
					f.Nombre, f.Hash, tipo, idGrupo, idUsuario).Error; err != nil {
					res.Errores = append(res.Errores, domain.ErrorFila{Fila: f.Num, Mensaje: "error al actualizar"})
					continue
				}
				res.Actualizados = append(res.Actualizados, idUsuario)
			}

			// Roles: se AGREGAN; no se quitan los que el admin ya haya asignado
			for _, rol := range roles {
				tx.Exec(`
					UPDATE OR INSERT INTO USUARIO_ROLES (ID_USUARIO, ROL)
					VALUES (CHAR_TO_UUID(?), ?) MATCHING (ID_USUARIO, ROL)`, idUsuario, rol)
			}
		}
		return nil
	})

	return res, err
}

// grupoID busca el grupo en el caché o en la base y lo crea si no existe
func grupoID(tx *gorm.DB, codigo, carrera string, cache map[string]string) (string, error) {
	if id, ok := cache[codigo]; ok {
		return id, nil
	}

	idCarrera, err := CarreraID(tx, carrera)
	if err != nil {
		return "", err
	}

	var id string
	tx.Raw(`SELECT UUID_TO_CHAR(ID) FROM GRUPOS WHERE CODIGO = ?`, codigo).Scan(&id)

	if id == "" {
		id = uuid.NewString()
		if err := tx.Exec(`
			INSERT INTO GRUPOS (ID, CODIGO, ID_CARRERA) VALUES (CHAR_TO_UUID(?), ?, CHAR_TO_UUID(?))`,
			id, codigo, idCarrera).Error; err != nil {
			return "", err
		}
	} else if idCarrera != nil {
		tx.Exec(`UPDATE GRUPOS SET ID_CARRERA = CHAR_TO_UUID(?) WHERE ID = CHAR_TO_UUID(?)`, idCarrera, id)
	}

	cache[codigo] = id
	return id, nil
}
