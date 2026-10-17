package repository

import (
	"errors"
	"strings"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrGrupoOcupado = errors.New("ya existe un grupo con ese código")
	ErrGrupoAlumnos = errors.New("el grupo tiene alumnos: muévelos o bórralos primero")
)

type GrupoRepository struct {
	db *gorm.DB
}

func NewGrupoRepository(db *gorm.DB) *GrupoRepository {
	return &GrupoRepository{db: db}
}

const columnasGrupo = `
	SELECT UUID_TO_CHAR(G.ID) ID, G.CODIGO, COALESCE(C.NOMBRE, '') CARRERA,
	       (SELECT COUNT(*) FROM USUARIOS A WHERE A.ID_GRUPO = G.ID) ALUMNOS
	FROM GRUPOS G
	LEFT JOIN CARRERAS C ON C.ID = G.ID_CARRERA`

// GetAll regresa todos los grupos (admin)
func (r *GrupoRepository) GetAll() []domain.Grupo {
	grupos := []domain.Grupo{}
	r.db.Raw(columnasGrupo + ` ORDER BY G.CODIGO`).Scan(&grupos)
	r.loadDocentes(grupos)
	return grupos
}

// GetByDocente regresa solo los grupos de un maestro
func (r *GrupoRepository) GetByDocente(idUsuario string) []domain.Grupo {
	grupos := []domain.Grupo{}
	r.db.Raw(columnasGrupo+`
		JOIN GRUPO_DOCENTES GD ON GD.ID_GRUPO = G.ID AND GD.ID_USUARIO = CHAR_TO_UUID(?)
		ORDER BY G.CODIGO`, idUsuario).Scan(&grupos)
	r.loadDocentes(grupos)
	return grupos
}

// loadDocentes llena los maestros de todos los grupos con UNA consulta
func (r *GrupoRepository) loadDocentes(grupos []domain.Grupo) {
	porID := make(map[string]*domain.Grupo, len(grupos)) // apuntadores: se llena el original
	for i := range grupos {
		grupos[i].Docentes = []domain.Persona{}
		porID[grupos[i].ID] = &grupos[i]
	}
	if len(grupos) == 0 {
		return
	}

	type fila struct {
		Grupo string `gorm:"column:GRUPO"`
		domain.Persona
	}
	var filas []fila

	r.db.Raw(`
		SELECT UUID_TO_CHAR(GD.ID_GRUPO) GRUPO, UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE
		FROM GRUPO_DOCENTES GD
		JOIN USUARIOS U ON U.ID = GD.ID_USUARIO
		ORDER BY U.NOMBRE`).Scan(&filas)

	for _, f := range filas {
		if g, ok := porID[f.Grupo]; ok {
			g.Docentes = append(g.Docentes, f.Persona)
		}
	}
}

// IsDocente dice si el maestro tiene asignado ese grupo
func (r *GrupoRepository) IsDocente(idGrupo, idUsuario string) bool {
	var n int
	r.db.Raw(`
		SELECT COUNT(*) FROM GRUPO_DOCENTES
		WHERE ID_GRUPO = CHAR_TO_UUID(?) AND ID_USUARIO = CHAR_TO_UUID(?)`, idGrupo, idUsuario).Scan(&n)
	return n > 0
}

func (r *GrupoRepository) Alumnos(idGrupo string) []domain.Persona {
	alumnos := []domain.Persona{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(ID) ID, MATRICULA, NOMBRE
		FROM USUARIOS
		WHERE ID_GRUPO = CHAR_TO_UUID(?)
		ORDER BY NOMBRE`, idGrupo).Scan(&alumnos)
	return alumnos
}

// CarreraID busca la carrera por nombre y la crea si no existe. "" = sin carrera.
// Se usa también al importar alumnos.
func CarreraID(tx *gorm.DB, nombre string) (*string, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return nil, nil
	}

	var id string
	tx.Raw(`SELECT UUID_TO_CHAR(ID) FROM CARRERAS WHERE UPPER(NOMBRE) = UPPER(?)`, nombre).Scan(&id)
	if id != "" {
		return &id, nil
	}

	id = uuid.NewString()
	if err := tx.Exec(`INSERT INTO CARRERAS (ID, NOMBRE) VALUES (CHAR_TO_UUID(?), ?)`, id, nombre).Error; err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *GrupoRepository) codigoOcupado(codigo, id string) bool {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM GRUPOS WHERE UPPER(CODIGO) = UPPER(?) AND ID <> CHAR_TO_UUID(?)`,
		codigo, id).Scan(&n)
	return n > 0
}

func (r *GrupoRepository) Create(g *domain.Grupo) error {
	g.Codigo = strings.ToUpper(strings.TrimSpace(g.Codigo))
	if r.codigoOcupado(g.Codigo, g.ID) {
		return ErrGrupoOcupado
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		carrera, err := CarreraID(tx, g.Carrera)
		if err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO GRUPOS (ID, CODIGO, ID_CARRERA)
			VALUES (CHAR_TO_UUID(?), ?, CHAR_TO_UUID(?))`, g.ID, g.Codigo, carrera).Error
	})
}

func (r *GrupoRepository) Update(g *domain.Grupo) error {
	g.Codigo = strings.ToUpper(strings.TrimSpace(g.Codigo))
	if r.codigoOcupado(g.Codigo, g.ID) {
		return ErrGrupoOcupado
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		carrera, err := CarreraID(tx, g.Carrera)
		if err != nil {
			return err
		}
		return tx.Exec(`
			UPDATE GRUPOS SET CODIGO = ?, ID_CARRERA = CHAR_TO_UUID(?)
			WHERE ID = CHAR_TO_UUID(?)`, g.Codigo, carrera, g.ID).Error
	})
}

// Delete solo si no tiene alumnos (sus maestros asignados se van en cascada)
func (r *GrupoRepository) Delete(id string) error {
	if len(r.Alumnos(id)) > 0 {
		return ErrGrupoAlumnos
	}
	return r.db.Exec(`DELETE FROM GRUPOS WHERE ID = CHAR_TO_UUID(?)`, id).Error
}

// SetDocentes reemplaza los maestros asignados al grupo
func (r *GrupoRepository) SetDocentes(idGrupo string, docentes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM GRUPO_DOCENTES WHERE ID_GRUPO = CHAR_TO_UUID(?)`, idGrupo).Error; err != nil {
			return err
		}
		for _, d := range docentes {
			if err := tx.Exec(`
				INSERT INTO GRUPO_DOCENTES (ID_GRUPO, ID_USUARIO)
				VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?))`, idGrupo, d).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GrupoRepository) Carreras() []domain.Carrera {
	carreras := []domain.Carrera{}
	r.db.Raw(`SELECT UUID_TO_CHAR(ID) ID, NOMBRE FROM CARRERAS ORDER BY NOMBRE`).Scan(&carreras)
	return carreras
}
