package repository

import (
	"strings"

	"raccoon-lab/backend/domain"

	"gorm.io/gorm"
)

const (
	maxFallos      = 5  // intentos fallidos antes de bloquear
	minutosBloqueo = 15 // minutos que dura el bloqueo
)

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// UserByMatricula busca al usuario con su grupo y roles. Regresa nil si no existe.
func (r *AuthRepository) UserByMatricula(matricula string) *domain.Usuario {
	var u domain.Usuario

	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE, U.PASSWORD, U.TIPO,
		       COALESCE(G.CODIGO, '') GRUPO, U.ACTIVO, U.ULTIMO_ACCESO
		FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		WHERE U.MATRICULA = ?`, strings.ToLower(matricula)).Scan(&u)

	if u.ID == "" {
		return nil
	}
	u.Roles = r.Roles(u.ID)
	return &u
}

// UserByID igual que UserByMatricula pero por ID (lo usa current-user-acc)
func (r *AuthRepository) UserByID(id string) *domain.Usuario {
	var u domain.Usuario

	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE, U.PASSWORD, U.TIPO,
		       COALESCE(G.CODIGO, '') GRUPO, U.ACTIVO, U.ULTIMO_ACCESO
		FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		WHERE U.ID = CHAR_TO_UUID(?)`, id).Scan(&u)

	if u.ID == "" {
		return nil
	}
	u.Roles = r.Roles(u.ID)
	return &u
}

// Roles de un usuario, del más alto al más bajo
func (r *AuthRepository) Roles(id string) []string {
	roles := []string{}

	r.db.Raw(`
		SELECT UR.ROL
		FROM USUARIO_ROLES UR
		JOIN ROLES R ON R.CLAVE = UR.ROL
		WHERE UR.ID_USUARIO = CHAR_TO_UUID(?)
		ORDER BY R.NIVEL DESC`, id).Scan(&roles)

	for i := range roles {
		roles[i] = strings.TrimSpace(roles[i])
	}
	return roles
}

func (r *AuthRepository) RegisterAccess(id string) error {
	return r.db.Exec(`
		UPDATE USUARIOS SET ULTIMO_ACCESO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`, id).Error
}

func (r *AuthRepository) UpdatePasswd(id string, hashPasswd string) error {
	return r.db.Exec(`
		UPDATE USUARIOS SET "PASSWORD" = ?
		WHERE ID = CHAR_TO_UUID(?)`, hashPasswd, id).Error
}

// ---------------------------------------------------------------------
// Bloqueo por intentos fallidos (tabla INTENTOS_LOGIN)
// Vive en Firebird para que las 2 réplicas de la API vean lo mismo
// ---------------------------------------------------------------------

// LockedSeconds regresa cuántos segundos le quedan de bloqueo (0 = no está bloqueado)
func (r *AuthRepository) LockedSeconds(matricula string) int {
	var segundos int

	r.db.Raw(`
		SELECT COALESCE(MAX(DATEDIFF(SECOND FROM CURRENT_TIMESTAMP TO BLOQUEADO_HASTA)), 0)
		FROM INTENTOS_LOGIN
		WHERE MATRICULA = ? AND BLOQUEADO_HASTA > CURRENT_TIMESTAMP`, strings.ToLower(matricula)).Scan(&segundos)

	return segundos
}

// LoginFailed suma un fallo. Regresa los intentos que quedan y,
// si llegó al límite, los segundos que dura el bloqueo.
func (r *AuthRepository) LoginFailed(matricula string) (restantes int, bloqueo int) {
	matricula = strings.ToLower(matricula)

	var fallos int
	r.db.Raw(`SELECT COALESCE(MAX(FALLOS), 0) FROM INTENTOS_LOGIN WHERE MATRICULA = ?`, matricula).Scan(&fallos)
	fallos++

	if fallos >= maxFallos {
		r.db.Exec(`
			UPDATE OR INSERT INTO INTENTOS_LOGIN (MATRICULA, FALLOS, BLOQUEADO_HASTA)
			VALUES (?, 0, DATEADD(? MINUTE TO CURRENT_TIMESTAMP))
			MATCHING (MATRICULA)`, matricula, minutosBloqueo)
		return 0, minutosBloqueo * 60
	}

	r.db.Exec(`
		UPDATE OR INSERT INTO INTENTOS_LOGIN (MATRICULA, FALLOS, BLOQUEADO_HASTA)
		VALUES (?, ?, NULL)
		MATCHING (MATRICULA)`, matricula, fallos)
	return maxFallos - fallos, 0
}

// LoginOK borra los intentos fallidos
func (r *AuthRepository) LoginOK(matricula string) {
	r.db.Exec(`DELETE FROM INTENTOS_LOGIN WHERE MATRICULA = ?`, strings.ToLower(matricula))
}

// ---------------------------------------------------------------------
// Admin general (sale del .env)
// ---------------------------------------------------------------------

// CreateAdmin crea el admin general si todavía no existe.
// Las 2 réplicas lo intentan al arrancar: si la otra ganó, no pasa nada.
func (r *AuthRepository) CreateAdmin(id, matricula, hashPasswd string) error {
	if r.UserByMatricula(matricula) != nil {
		return nil
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO USUARIOS (ID, MATRICULA, NOMBRE, "PASSWORD", TIPO)
			VALUES (CHAR_TO_UUID(?), ?, 'Administrador General', ?, 'admins')`,
			id, strings.ToLower(matricula), hashPasswd).Error; err != nil {
			return err
		}
		// Los roles salen del catálogo TIPO_ROLES
		return tx.Exec(`
			INSERT INTO USUARIO_ROLES (ID_USUARIO, ROL)
			SELECT CHAR_TO_UUID(?), ROL FROM TIPO_ROLES WHERE TIPO = 'admins'`, id).Error
	})

	if err != nil && r.UserByMatricula(matricula) != nil {
		return nil // lo creó la otra réplica al mismo tiempo
	}
	return err
}
