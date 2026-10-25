package repository

import (
	"errors"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrDespliegueEnCurso = errors.New("ya hay un despliegue en curso en tu laboratorio; espera a que termine")

type DespliegueRepository struct {
	db *gorm.DB
}

func NewDespliegueRepository(db *gorm.DB) *DespliegueRepository {
	return &DespliegueRepository{db: db}
}

// Reserve registra un despliegue nuevo en PROCESANDO. Solo puede haber UNO
// por laboratorio: LAB_ACTIVO es UNIQUE, así que si otra réplica ya empezó
// uno en el mismo lab, el INSERT falla.
func (r *DespliegueRepository) Reserve(idLab, idUsuario, origen, proyecto string) (string, error) {
	// Si una réplica murió a medio despliegue, su latido se detiene: se da por fallido
	r.db.Exec(`
		UPDATE DESPLIEGUES SET ESTADO = 'ERROR', LAB_ACTIVO = NULL, FIN = CURRENT_TIMESTAMP,
		       MENSAJE = 'Se interrumpió (el servidor se reinició); vuelve a desplegar'
		WHERE ESTADO = 'PROCESANDO' AND ACTUALIZADO < DATEADD(-3 MINUTE TO CURRENT_TIMESTAMP)`)

	id := uuid.NewString()
	err := r.db.Exec(`
		INSERT INTO DESPLIEGUES (ID, ID_LABORATORIO, LAB_ACTIVO, ID_USUARIO, ORIGEN, PROYECTO, PASO)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?, 'Preparando')`,
		id, idLab, idLab, idUsuario, origen, proyecto).Error
	if err != nil {
		var activos int
		r.db.Raw(`SELECT COUNT(*) FROM DESPLIEGUES WHERE LAB_ACTIVO = CHAR_TO_UUID(?)`, idLab).Scan(&activos)
		if activos > 0 {
			return "", ErrDespliegueEnCurso
		}
		return "", err
	}
	return id, nil
}

// Progress guarda el avance y renueva el latido
func (r *DespliegueRepository) Progress(id string, porcentaje int, paso string) {
	if len(paso) > 120 {
		paso = paso[:120]
	}
	r.db.Exec(`
		UPDATE DESPLIEGUES SET PORCENTAJE = ?, PASO = ?, ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`, porcentaje, paso, id)
}

// Heartbeat solo renueva el latido (cuando un paso tarda mucho)
func (r *DespliegueRepository) Heartbeat(id string) {
	r.db.Exec(`UPDATE DESPLIEGUES SET ACTUALIZADO = CURRENT_TIMESTAMP WHERE ID = CHAR_TO_UUID(?)`, id)
}

// Finish cierra el despliegue (TERMINADO o ERROR) y libera el laboratorio
func (r *DespliegueRepository) Finish(id, estado, mensaje string) {
	if len(mensaje) > 1000 {
		mensaje = mensaje[:1000]
	}
	r.db.Exec(`
		UPDATE DESPLIEGUES SET ESTADO = ?, MENSAJE = ?, LAB_ACTIVO = NULL, FIN = CURRENT_TIMESTAMP,
		       ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`, estado, mensaje, id)
	if estado == "TERMINADO" {
		r.db.Exec(`UPDATE DESPLIEGUES SET PORCENTAJE = 100, PASO = 'Terminado' WHERE ID = CHAR_TO_UUID(?)`, id)
	}
}

// Last: el despliegue más reciente del laboratorio (nil si nunca hubo)
func (r *DespliegueRepository) Last(idLab string) *domain.Despliegue {
	var d domain.Despliegue
	r.db.Raw(`
		SELECT FIRST 1 UUID_TO_CHAR(ID) ID, ORIGEN, PROYECTO, ESTADO, PORCENTAJE,
		       COALESCE(PASO, '') PASO, COALESCE(MENSAJE, '') MENSAJE, INICIO
		FROM DESPLIEGUES
		WHERE ID_LABORATORIO = CHAR_TO_UUID(?)
		ORDER BY INICIO DESC`, idLab).Scan(&d)
	if d.ID == "" {
		return nil
	}
	return &d
}
