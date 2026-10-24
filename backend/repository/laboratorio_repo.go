package repository

import (
	"crypto/rand"
	"errors"
	"strings"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrYaEnLaboratorio  = errors.New("ya estás en un laboratorio: límpialo o sal del equipo primero")
	ErrSinPuertos       = errors.New("se alcanzó el máximo de laboratorios abiertos; intenta más tarde")
	ErrCodigoInvalido   = errors.New("no existe un laboratorio con ese código")
	ErrSinLaboratorio   = errors.New("no tienes un laboratorio")
	ErrNoEsDueno        = errors.New("solo el dueño del laboratorio puede hacer esto")
	ErrNoEsEquipo       = errors.New("el laboratorio es individual: créalo en modo equipo para invitar")
	ErrInvitacion       = errors.New("la invitación ya no es válida")
	ErrYaInvitado       = errors.New("ya lo invitaste, espera su respuesta")
	ErrCompaneroOcupado = errors.New("ese compañero ya está en un laboratorio")
)

type LaboratorioRepository struct {
	db *gorm.DB
}

func NewLaboratorioRepository(db *gorm.DB) *LaboratorioRepository {
	return &LaboratorioRepository{db: db}
}

const columnasLab = `
	SELECT UUID_TO_CHAR(L.ID) ID, UUID_TO_CHAR(L.ID_DUENO) ID_DUENO, L.CONTENEDOR, L.PUERTO_BASE,
	       L.TOKEN, COALESCE(L.CODIGO_EQUIPO, '') CODIGO_EQUIPO, L.ESTADO,
	       COALESCE(L.MENSAJE, '') MENSAJE, L.INICIO, U.NOMBRE DUENO, U.MATRICULA USUARIO
	FROM LABORATORIOS L
	JOIN USUARIOS U ON U.ID = L.ID_DUENO`

// aleatorio genera textos al azar con letras y números (sin 0/O ni 1/I para no confundir)
func aleatorio(n int) string {
	const letras = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letras[int(b[i])%len(letras)]
	}
	return string(b)
}

// ByUser busca el laboratorio del usuario, como dueño o como miembro.
// Regresa (nil, false) si no está en ninguno.
func (r *LaboratorioRepository) ByUser(idUsuario string) (*domain.Laboratorio, bool) {
	// Si una réplica murió mientras creaba un laboratorio, se marca como fallido
	r.db.Exec(`
		UPDATE LABORATORIOS SET ESTADO = 'ERROR', MENSAJE = 'Se interrumpió la creación; límpialo e intenta de nuevo'
		WHERE ESTADO = 'CREANDO' AND INICIO < DATEADD(-10 MINUTE TO CURRENT_TIMESTAMP)`)

	var lab domain.Laboratorio
	r.db.Raw(columnasLab+` WHERE L.ID_DUENO = CHAR_TO_UUID(?)`, idUsuario).Scan(&lab)
	if lab.ID != "" {
		return &lab, true
	}

	r.db.Raw(columnasLab+`
		JOIN LABORATORIO_MIEMBROS M ON M.ID_LABORATORIO = L.ID
		WHERE M.ID_USUARIO = CHAR_TO_UUID(?)`, idUsuario).Scan(&lab)
	if lab.ID != "" {
		return &lab, false
	}
	return nil, false
}

func (r *LaboratorioRepository) ByID(id string) *domain.Laboratorio {
	var lab domain.Laboratorio
	r.db.Raw(columnasLab+` WHERE L.ID = CHAR_TO_UUID(?)`, id).Scan(&lab)
	if lab.ID == "" {
		return nil
	}
	return &lab
}

func (r *LaboratorioRepository) estaEnUno(idUsuario string) bool {
	var n int
	r.db.Raw(`
		SELECT (SELECT COUNT(*) FROM LABORATORIOS WHERE ID_DUENO = CHAR_TO_UUID(?)) +
		       (SELECT COUNT(*) FROM LABORATORIO_MIEMBROS WHERE ID_USUARIO = CHAR_TO_UUID(?))
		FROM RDB$DATABASE`, idUsuario, idUsuario).Scan(&n)
	return n > 0
}

// Reserve aparta el siguiente bloque de puertos libre y registra el laboratorio
// en estado CREANDO. Los UNIQUE de la tabla evitan que 2 réplicas tomen el mismo.
func (r *LaboratorioRepository) Reserve(idDueno string, colaborativo bool, puertoInicio, puertosPorLab, maximo int) (*domain.Laboratorio, error) {
	if r.estaEnUno(idDueno) {
		return nil, ErrYaEnLaboratorio
	}

	for intento := 0; intento < 5; intento++ {
		var usados []int
		r.db.Raw(`SELECT PUERTO_BASE FROM LABORATORIOS`).Scan(&usados)
		ocupado := make(map[int]bool, len(usados))
		for _, p := range usados {
			ocupado[p] = true
		}

		base := 0
		for k := 0; k < maximo; k++ {
			if p := puertoInicio + k*puertosPorLab; !ocupado[p] {
				base = p
				break
			}
		}
		if base == 0 {
			return nil, ErrSinPuertos
		}

		var codigo *string
		if colaborativo {
			c := aleatorio(6)
			codigo = &c
		}

		lab := &domain.Laboratorio{
			ID:         uuid.NewString(),
			IDDueno:    idDueno,
			Contenedor: "rl-lab-" + strings.ToLower(strings.ReplaceAll(idDueno, "-", "")[:12]),
			PuertoBase: base,
			Token:      aleatorio(8),
			Estado:     "CREANDO",
		}
		if codigo != nil {
			lab.Codigo = *codigo
		}

		err := r.db.Exec(`
			INSERT INTO LABORATORIOS (ID, ID_DUENO, CONTENEDOR, PUERTO_BASE, TOKEN, CODIGO_EQUIPO)
			VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?, ?, ?)`,
			lab.ID, lab.IDDueno, lab.Contenedor, lab.PuertoBase, lab.Token, codigo).Error
		if err == nil {
			return lab, nil
		}
		if r.estaEnUno(idDueno) {
			return nil, ErrYaEnLaboratorio
		}
		// Otra réplica tomó el mismo puerto al mismo tiempo: se intenta con el siguiente
	}
	return nil, ErrSinPuertos
}

func (r *LaboratorioRepository) SetEstado(id, estado, mensaje string) {
	if len(mensaje) > 300 {
		mensaje = mensaje[:300]
	}
	r.db.Exec(`UPDATE LABORATORIOS SET ESTADO = ?, MENSAJE = ? WHERE ID = CHAR_TO_UUID(?)`, estado, mensaje, id)
}

// Delete borra el laboratorio; miembros e invitaciones se van en cascada
func (r *LaboratorioRepository) Delete(id string) error {
	return r.db.Exec(`DELETE FROM LABORATORIOS WHERE ID = CHAR_TO_UUID(?)`, id).Error
}

func (r *LaboratorioRepository) Miembros(id string) []domain.Persona {
	miembros := []domain.Persona{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE
		FROM LABORATORIO_MIEMBROS M
		JOIN USUARIOS U ON U.ID = M.ID_USUARIO
		WHERE M.ID_LABORATORIO = CHAR_TO_UUID(?)
		ORDER BY M.UNIDO`, id).Scan(&miembros)
	return miembros
}

// Join agrega al usuario al laboratorio de equipo con ese código
func (r *LaboratorioRepository) Join(idUsuario, codigo string) error {
	if r.estaEnUno(idUsuario) {
		return ErrYaEnLaboratorio
	}

	var id string
	r.db.Raw(`SELECT UUID_TO_CHAR(ID) FROM LABORATORIOS WHERE CODIGO_EQUIPO = ?`,
		strings.ToUpper(strings.TrimSpace(codigo))).Scan(&id)
	if id == "" {
		return ErrCodigoInvalido
	}
	return r.addMember(r.db, id, idUsuario)
}

func (r *LaboratorioRepository) addMember(tx *gorm.DB, idLab, idUsuario string) error {
	err := tx.Exec(`
		INSERT INTO LABORATORIO_MIEMBROS (ID_LABORATORIO, ID_USUARIO)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?))`, idLab, idUsuario).Error
	if err != nil && r.estaEnUno(idUsuario) {
		return ErrYaEnLaboratorio
	}
	return err
}

func (r *LaboratorioRepository) Leave(idUsuario string) error {
	return r.db.Exec(`DELETE FROM LABORATORIO_MIEMBROS WHERE ID_USUARIO = CHAR_TO_UUID(?)`, idUsuario).Error
}

// Companeros: alumnos del mismo grupo que todavía no están en un laboratorio
func (r *LaboratorioRepository) Companeros(idUsuario string) []domain.Persona {
	companeros := []domain.Persona{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE
		FROM USUARIOS U
		JOIN USUARIOS YO ON YO.ID = CHAR_TO_UUID(?) AND YO.ID_GRUPO = U.ID_GRUPO
		WHERE U.ID <> YO.ID AND U.ACTIVO
		  AND NOT EXISTS (SELECT 1 FROM LABORATORIOS L WHERE L.ID_DUENO = U.ID)
		  AND NOT EXISTS (SELECT 1 FROM LABORATORIO_MIEMBROS M WHERE M.ID_USUARIO = U.ID)
		ORDER BY U.NOMBRE`, idUsuario).Scan(&companeros)
	return companeros
}

// Invite crea una invitación del dueño a un compañero
func (r *LaboratorioRepository) Invite(idDueno, idReceptor string) error {
	lab, dueno := r.ByUser(idDueno)
	if lab == nil {
		return ErrSinLaboratorio
	}
	if !dueno {
		return ErrNoEsDueno
	}
	if lab.Codigo == "" {
		return ErrNoEsEquipo
	}
	if r.estaEnUno(idReceptor) {
		return ErrCompaneroOcupado
	}

	var pendientes int
	r.db.Raw(`
		SELECT COUNT(*) FROM INVITACIONES
		WHERE ID_LABORATORIO = CHAR_TO_UUID(?) AND ID_RECEPTOR = CHAR_TO_UUID(?) AND ESTADO = 'PENDIENTE'`,
		lab.ID, idReceptor).Scan(&pendientes)
	if pendientes > 0 {
		return ErrYaInvitado
	}

	return r.db.Exec(`
		INSERT INTO INVITACIONES (ID, ID_LABORATORIO, ID_EMISOR, ID_RECEPTOR)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?))`,
		uuid.NewString(), lab.ID, idDueno, idReceptor).Error
}

// Invitations: las invitaciones pendientes que recibió el usuario
func (r *LaboratorioRepository) Invitations(idUsuario string) []domain.Invitacion {
	invitaciones := []domain.Invitacion{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(I.ID) ID, U.NOMBRE EMISOR, U.MATRICULA, I.CREADA
		FROM INVITACIONES I
		JOIN USUARIOS U ON U.ID = I.ID_EMISOR
		WHERE I.ID_RECEPTOR = CHAR_TO_UUID(?) AND I.ESTADO = 'PENDIENTE'
		ORDER BY I.CREADA DESC`, idUsuario).Scan(&invitaciones)
	return invitaciones
}

// Answer acepta o rechaza una invitación
func (r *LaboratorioRepository) Answer(idInvitacion, idUsuario string, aceptar bool) error {
	var idLab string
	r.db.Raw(`
		SELECT UUID_TO_CHAR(ID_LABORATORIO) FROM INVITACIONES
		WHERE ID = CHAR_TO_UUID(?) AND ID_RECEPTOR = CHAR_TO_UUID(?) AND ESTADO = 'PENDIENTE'`,
		idInvitacion, idUsuario).Scan(&idLab)
	if idLab == "" {
		return ErrInvitacion
	}

	if !aceptar {
		return r.db.Exec(`UPDATE INVITACIONES SET ESTADO = 'RECHAZADA' WHERE ID = CHAR_TO_UUID(?)`, idInvitacion).Error
	}

	if r.estaEnUno(idUsuario) {
		return ErrYaEnLaboratorio
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.addMember(tx, idLab, idUsuario); err != nil {
			return err
		}
		return tx.Exec(`UPDATE INVITACIONES SET ESTADO = 'ACEPTADA' WHERE ID = CHAR_TO_UUID(?)`, idInvitacion).Error
	})
}

// All: todos los laboratorios (monitor del admin)
func (r *LaboratorioRepository) All() []domain.LabActivo {
	labs := []domain.LabActivo{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(L.ID) ID, U.MATRICULA, U.NOMBRE, COALESCE(G.CODIGO, '') GRUPO,
		       L.PUERTO_BASE, COALESCE(L.CODIGO_EQUIPO, '') CODIGO_EQUIPO, L.ESTADO, L.INICIO,
		       (SELECT COUNT(*) FROM LABORATORIO_MIEMBROS M WHERE M.ID_LABORATORIO = L.ID) MIEMBROS
		FROM LABORATORIOS L
		JOIN USUARIOS U ON U.ID = L.ID_DUENO
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		ORDER BY L.INICIO`).Scan(&labs)
	return labs
}
