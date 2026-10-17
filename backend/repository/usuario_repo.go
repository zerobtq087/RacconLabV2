package repository

import (
	"errors"
	"strings"

	"raccoon-lab/backend/domain"

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
		       COALESCE(G.CODIGO, '') GRUPO, U.ACTIVO, U.ULTIMO_ACCESO
		FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
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

// GetByID regresa el usuario (sin roles) o nil
func (r *UsuarioRepository) GetByID(id string) *domain.Usuario {
	var u domain.Usuario

	r.db.Raw(`
		SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE, U.TIPO,
		       COALESCE(G.CODIGO, '') GRUPO, U.ACTIVO
		FROM USUARIOS U
		LEFT JOIN GRUPOS G ON G.ID = U.ID_GRUPO
		WHERE U.ID = CHAR_TO_UUID(?)`, id).Scan(&u)

	if u.ID == "" {
		return nil
	}
	return &u
}

// Update guarda datos, grupo, activo y roles en una transacción
func (r *UsuarioRepository) Update(u *domain.Usuario) error {
	matricula := strings.ToLower(strings.TrimSpace(u.Matricula))

	var ocupada int
	r.db.Raw(`SELECT COUNT(*) FROM USUARIOS WHERE MATRICULA = ? AND ID <> CHAR_TO_UUID(?)`,
		matricula, u.ID).Scan(&ocupada)
	if ocupada > 0 {
		return ErrMatriculaOcupada
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// El grupo llega como código (IRD-71); se guarda su ID
		if err := tx.Exec(`
			UPDATE USUARIOS SET
				MATRICULA = ?,
				NOMBRE = ?,
				ACTIVO = ?,
				ID_GRUPO = (SELECT ID FROM GRUPOS WHERE CODIGO = ?)
			WHERE ID = CHAR_TO_UUID(?)`,
			matricula, strings.TrimSpace(u.Nombre), u.Activo, u.Grupo, u.ID).Error; err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM USUARIO_ROLES WHERE ID_USUARIO = CHAR_TO_UUID(?)`, u.ID).Error; err != nil {
			return err
		}
		for _, rol := range u.Roles {
			if err := tx.Exec(`
				INSERT INTO USUARIO_ROLES (ID_USUARIO, ROL) VALUES (CHAR_TO_UUID(?), ?)`,
				u.ID, rol).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *UsuarioRepository) UpdatePasswd(id, hashPasswd string) error {
	return r.db.Exec(`UPDATE USUARIOS SET "PASSWORD" = ? WHERE ID = CHAR_TO_UUID(?)`, hashPasswd, id).Error
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
