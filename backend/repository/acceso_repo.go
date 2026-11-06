package repository

import (
	"time"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AccesoRepository: qué IPs pueden usar los puertos de cada lab.
// Una IP entra al firewall si tiene una SESIÓN web vigente y su usuario es
// dueño o miembro del lab, o es un docente supervisándolo (por tiempo limitado).
// Todo está en Firebird para que las 2 réplicas den la misma lista.
type AccesoRepository struct {
	db *gorm.DB
}

func NewAccesoRepository(db *gorm.DB) *AccesoRepository {
	return &AccesoRepository{db: db}
}

// GuardarSesion registra (o actualiza) la IP de una sesión web
func (r *AccesoRepository) GuardarSesion(idSesion, idUsuario, ip string) {
	r.db.Exec(`
		UPDATE OR INSERT INTO SESIONES_IP (ID_SESION, ID_USUARIO, IP, VISTA)
		VALUES (?, CHAR_TO_UUID(?), ?, CURRENT_TIMESTAMP) MATCHING (ID_SESION)`, idSesion, idUsuario, ip)
}

func (r *AccesoRepository) QuitarSesion(idSesion string) {
	r.db.Exec(`DELETE FROM SESIONES_IP WHERE ID_SESION = ?`, idSesion)
}

// QuitarSesiones borra todas las sesiones del usuario menos "excepto"
func (r *AccesoRepository) QuitarSesiones(idUsuario, excepto string) {
	r.db.Exec(`DELETE FROM SESIONES_IP WHERE ID_USUARIO = CHAR_TO_UUID(?) AND ID_SESION <> ?`, idUsuario, excepto)
}

// Accesos: las IPs y el puerto base de cada lab al que tienen acceso.
// Las sesiones vencidas (inactividad o tiempo máximo, igual que en Redis) no cuentan.
func (r *AccesoRepository) Accesos(inactividad, maximo time.Duration) []domain.Acceso {
	desde := time.Now().Add(-inactividad)
	creadaDesde := time.Now().Add(-maximo)

	// Limpieza de sesiones vencidas y supervisiones terminadas
	r.db.Exec(`DELETE FROM SESIONES_IP WHERE VISTA < ? OR CREADA < ?`, desde, creadaDesde)
	r.db.Exec(`DELETE FROM SUPERVISIONES_ACTIVAS WHERE HASTA < CURRENT_TIMESTAMP`)

	accesos := []domain.Acceso{}
	r.db.Raw(`
		SELECT S.IP, L.PUERTO_BASE FROM SESIONES_IP S
		JOIN LABORATORIOS L ON L.ID_DUENO = S.ID_USUARIO
		UNION
		SELECT S.IP, L.PUERTO_BASE FROM SESIONES_IP S
		JOIN LABORATORIO_MIEMBROS M ON M.ID_USUARIO = S.ID_USUARIO
		JOIN LABORATORIOS L ON L.ID = M.ID_LABORATORIO
		UNION
		SELECT S.IP, L.PUERTO_BASE FROM SESIONES_IP S
		JOIN SUPERVISIONES_ACTIVAS A ON A.ID_USUARIO = S.ID_USUARIO
		JOIN LABORATORIOS L ON L.ID = A.ID_LABORATORIO
		ORDER BY 2, 1`).Scan(&accesos)
	return accesos
}

// Supervisar abre (o renueva) los puertos de un lab para un docente durante "duracion"
func (r *AccesoRepository) Supervisar(idUsuario, idLab string, duracion time.Duration) {
	r.db.Exec(`
		UPDATE OR INSERT INTO SUPERVISIONES_ACTIVAS (ID_USUARIO, ID_LABORATORIO, HASTA)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?) MATCHING (ID_USUARIO, ID_LABORATORIO)`,
		idUsuario, idLab, time.Now().Add(duracion))
}

func (r *AccesoRepository) DejarDeSupervisar(idUsuario, idLab string) {
	r.db.Exec(`DELETE FROM SUPERVISIONES_ACTIVAS
		WHERE ID_USUARIO = CHAR_TO_UUID(?) AND ID_LABORATORIO = CHAR_TO_UUID(?)`, idUsuario, idLab)
}

// Bitacora: lo que consultó un docente en el lab de un alumno
func (r *AccesoRepository) Bitacora(idUsuario, idAlumno, accion, detalle string) {
	if r := []rune(detalle); len(r) > 200 {
		detalle = string(r[:200])
	}
	r.db.Exec(`
		INSERT INTO SUPERVISIONES (ID, ID_USUARIO, ID_ALUMNO, ACCION, DETALLE)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?)`,
		uuid.NewString(), idUsuario, idAlumno, accion, detalle)
}

// Bitacoras: las últimas consultas (de un docente, o de todos con "")
func (r *AccesoRepository) Bitacoras(idUsuario string, maximo int) []domain.EntradaBitacora {
	consulta := `
		SELECT FIRST ? S.FECHA, COALESCE(D.NOMBRE, '') DOCENTE,
		       COALESCE(A.MATRICULA || ' - ' || A.NOMBRE, '') ALUMNO, S.ACCION, COALESCE(S.DETALLE, '') DETALLE
		FROM SUPERVISIONES S
		LEFT JOIN USUARIOS D ON D.ID = S.ID_USUARIO
		LEFT JOIN USUARIOS A ON A.ID = S.ID_ALUMNO`
	lista := []domain.EntradaBitacora{}
	if idUsuario == "" {
		r.db.Raw(consulta+` ORDER BY S.FECHA DESC`, maximo).Scan(&lista)
	} else {
		r.db.Raw(consulta+` WHERE S.ID_USUARIO = CHAR_TO_UUID(?) ORDER BY S.FECHA DESC`, maximo, idUsuario).Scan(&lista)
	}
	return lista
}
