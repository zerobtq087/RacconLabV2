package repository

import (
	"errors"
	"strings"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMatriculaOcupada = errors.New("esa matrícula ya la tiene otro usuario")

type UsuarioRepository struct {
	db *gorm.DB
}

func NewUsuarioRepository(db *gorm.DB) *UsuarioRepository {
	return &UsuarioRepository{db: db}
}

// GetPage regresa solo la página que pide la tabla (sirve para miles de alumnos)
func (r *UsuarioRepository) GetPage(f *domain.FiltroUsuarios) *domain.UsuarioPagina {
	where := []string{"1 = 1"}
	args := []any{}

	if f.Buscar != "" {
		where = append(where, "(U.MATRICULA CONTAINING ? OR U.NOMBRE CONTAINING ?)")
		args = append(args, f.Buscar, f.Buscar)
	}
	if f.Tipo != "" {
		where = append(where, "U.TIPO = ?")
		args = append(args, f.Tipo)
	}
	if f.Grupo != "" {
		where = append(where, "G.CODIGO = ?")
		args = append(args, f.Grupo)
	}
	filtro := strings.Join(where, " AND ")

	pagina := &domain.UsuarioPagina{Usuarios: []domain.Usuario{}}

	r.db.Raw(`
		SELECT COUNT(*) FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		WHERE `+filtro, args...).Scan(&pagina.Total)

	args = append(args, (f.Pagina-1)*f.PorPagina, f.PorPagina)
	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE, U.TIPO,
		       COALESCE(G.CODIGO, '') GRUPO, COALESCE(C.NOMBRE, '') CARRERA, U.ACTIVO, U.ULTIMO_ACCESO
		FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		LEFT JOIN CARRERAS C ON C.ID = G.ID_CARRERA
		WHERE `+filtro+`
		ORDER BY U.TIPO DESC, U.MATRICULA
		OFFSET ? ROWS FETCH NEXT ? ROWS ONLY`, args...).Scan(&pagina.Usuarios)

	r.loadRoles(pagina.Usuarios)
	return pagina
}

// loadRoles llena los roles de varios usuarios con UNA sola consulta
func (r *UsuarioRepository) loadRoles(usuarios []domain.Usuario) {
	if len(usuarios) == 0 {
		return
	}

	marcas := make([]string, len(usuarios))
	ids := make([]any, len(usuarios))
	porID := make(map[string]*domain.Usuario, len(usuarios)) // apuntadores: se llena el original
	for i := range usuarios {
		marcas[i] = "CHAR_TO_UUID(?)"
		ids[i] = usuarios[i].ID
		usuarios[i].Roles = []string{}
		porID[usuarios[i].ID] = &usuarios[i]
	}

	type fila struct {
		ID  string `gorm:"column:ID"`
		Rol string `gorm:"column:ROL"`
	}
	var filas []fila

	r.db.Raw(`
		SELECT UUID_TO_CHAR(UR.ID_USUARIO) ID, UR.ROL
		FROM USUARIO_ROLES UR
		JOIN ROLES R ON R.CLAVE = UR.ROL
		WHERE UR.ID_USUARIO IN (`+strings.Join(marcas, ",")+`)
		ORDER BY R.NIVEL DESC`, ids...).Scan(&filas)

	for _, f := range filas {
		if u, ok := porID[f.ID]; ok {
			u.Roles = append(u.Roles, strings.TrimSpace(f.Rol))
		}
	}
}

// GetByID regresa el usuario con sus roles, o nil
func (r *UsuarioRepository) GetByID(id string) *domain.Usuario {
	u := []domain.Usuario{{}}

	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE, U.TIPO,
		       COALESCE(G.CODIGO, '') GRUPO, COALESCE(C.NOMBRE, '') CARRERA, U.ACTIVO
		FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		LEFT JOIN CARRERAS C ON C.ID = G.ID_CARRERA
		WHERE U.ID = CHAR_TO_UUID(?)`, id).Scan(&u[0])

	if u[0].ID == "" {
		return nil
	}
	r.loadRoles(u)
	return &u[0]
}

// Tipos: el catálogo TIPOS_USUARIO con los roles que da cada tipo (TIPO_ROLES)
func (r *UsuarioRepository) Tipos() []domain.TipoUsuario {
	tipos := []domain.TipoUsuario{}
	r.db.Raw(`SELECT CLAVE, DESCRIPCION FROM TIPOS_USUARIO ORDER BY CLAVE`).Scan(&tipos)

	for i := range tipos {
		tipos[i].Clave = strings.TrimSpace(tipos[i].Clave)
		tipos[i].Roles = []string{}
		r.db.Raw(`
			SELECT TR.ROL FROM TIPO_ROLES TR
			JOIN ROLES R ON R.CLAVE = TR.ROL
			WHERE TR.TIPO = ?
			ORDER BY R.NIVEL DESC`, tipos[i].Clave).Scan(&tipos[i].Roles)
		for k := range tipos[i].Roles {
			tipos[i].Roles[k] = strings.TrimSpace(tipos[i].Roles[k])
		}
	}
	return tipos
}

func (r *UsuarioRepository) matriculaOcupada(matricula, id string) bool {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM USUARIOS WHERE MATRICULA = ? AND ID <> CHAR_TO_UUID(?)`,
		matricula, id).Scan(&n)
	return n > 0
}

// guardarRoles reemplaza los roles del usuario
func guardarRoles(tx *gorm.DB, id string, roles []string) error {
	if err := tx.Exec(`DELETE FROM USUARIO_ROLES WHERE ID_USUARIO = CHAR_TO_UUID(?)`, id).Error; err != nil {
		return err
	}
	for _, rol := range roles {
		if err := tx.Exec(`INSERT INTO USUARIO_ROLES (ID_USUARIO, ROL) VALUES (CHAR_TO_UUID(?), ?)`,
			id, rol).Error; err != nil {
			return err
		}
	}
	return nil
}

// grupoDeForm: el grupo del alumno (se crea si no existe, como al importar); nil si no es alumno
func grupoDeForm(tx *gorm.DB, f *domain.UsuarioForm) (*string, error) {
	if f.Grupo == "" {
		return nil, nil
	}
	id, err := grupoID(tx, f.Grupo, f.Carrera, map[string]string{})
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// Create da de alta un usuario con sus roles (el formulario ya viene validado)
func (r *UsuarioRepository) Create(f *domain.UsuarioForm, hashPasswd string) error {
	id := uuid.NewString()
	if r.matriculaOcupada(f.Matricula, id) {
		return ErrMatriculaOcupada
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		idGrupo, err := grupoDeForm(tx, f)
		if err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO USUARIOS (ID, MATRICULA, NOMBRE, "PASSWORD", TIPO, ID_GRUPO, ACTIVO)
			VALUES (CHAR_TO_UUID(?), ?, ?, ?, ?, CHAR_TO_UUID(?), ?)`,
			id, f.Matricula, f.Nombre, hashPasswd, f.Tipo, idGrupo, f.Activo).Error; err != nil {
			return err
		}
		return guardarRoles(tx, id, f.Roles)
	})
}

// Update guarda datos, tipo, grupo, activo, roles y (si viene) la contraseña en una transacción
func (r *UsuarioRepository) Update(id string, f *domain.UsuarioForm, hashPasswd string) error {
	if r.matriculaOcupada(f.Matricula, id) {
		return ErrMatriculaOcupada
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		idGrupo, err := grupoDeForm(tx, f)
		if err != nil {
			return err
		}
		if err := tx.Exec(`
			UPDATE USUARIOS SET MATRICULA = ?, NOMBRE = ?, TIPO = ?, ACTIVO = ?, ID_GRUPO = CHAR_TO_UUID(?)
			WHERE ID = CHAR_TO_UUID(?)`,
			f.Matricula, f.Nombre, f.Tipo, f.Activo, idGrupo, id).Error; err != nil {
			return err
		}
		if hashPasswd != "" {
			if err := tx.Exec(`UPDATE USUARIOS SET "PASSWORD" = ? WHERE ID = CHAR_TO_UUID(?)`,
				hashPasswd, id).Error; err != nil {
				return err
			}
		}
		return guardarRoles(tx, id, f.Roles)
	})
}

// Delete borra al usuario; sus roles y grupos asignados se van en cascada
func (r *UsuarioRepository) Delete(id string) error {
	return r.db.Exec(`DELETE FROM USUARIOS WHERE ID = CHAR_TO_UUID(?)`, id).Error
}

// Maestros: usuarios con rol PROFESOR (para asignarlos a grupos)
func (r *UsuarioRepository) Maestros() []domain.Persona {
	maestros := []domain.Persona{}

	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE
		FROM USUARIOS U
		JOIN USUARIO_ROLES UR ON UR.ID_USUARIO = U.ID AND UR.ROL = 'PROFESOR'
		WHERE U.ACTIVO
		ORDER BY U.NOMBRE`).Scan(&maestros)

	return maestros
}
