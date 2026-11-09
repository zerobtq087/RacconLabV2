package repository

import (
	"strings"

	"raccoon-lab/backend/domain"

	"gorm.io/gorm"
)

// InicioRepository: los conteos del tablero de Inicio
type InicioRepository struct {
	db *gorm.DB
}

func NewInicioRepository(db *gorm.DB) *InicioRepository {
	return &InicioRepository{db: db}
}

// FirebirdResponde: una consulta mínima (estado de los servicios)
func (r *InicioRepository) FirebirdResponde() bool {
	var n int
	return r.db.Raw(`SELECT 1 FROM RDB$DATABASE`).Scan(&n).Error == nil && n == 1
}

// GruposDelDocente: sus grupos con cuántos alumnos tienen el lab encendido
// (como dueños o miembros de un lab CORRIENDO)
func (r *InicioRepository) GruposDelDocente(idUsuario string) []domain.ResumenGrupo {
	grupos := []domain.ResumenGrupo{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(G.ID) ID, G.CODIGO, COALESCE(C.NOMBRE, '') CARRERA,
		       (SELECT COUNT(*) FROM USUARIOS A WHERE A.ID_GRUPO = G.ID AND A.TIPO = 'alumnos') ALUMNOS,
		       (SELECT COUNT(*) FROM USUARIOS A WHERE A.ID_GRUPO = G.ID AND A.TIPO = 'alumnos' AND EXISTS (
		            SELECT 1 FROM LABORATORIOS L WHERE L.ESTADO = 'CORRIENDO' AND (L.ID_DUENO = A.ID OR EXISTS (
		                SELECT 1 FROM LABORATORIO_MIEMBROS M WHERE M.ID_LABORATORIO = L.ID AND M.ID_USUARIO = A.ID)))) LABS
		FROM GRUPOS G
		JOIN GRUPO_DOCENTES GD ON GD.ID_GRUPO = G.ID AND GD.ID_USUARIO = CHAR_TO_UUID(?)
		LEFT JOIN CARRERAS C ON C.ID = G.ID_CARRERA
		ORDER BY G.CODIGO`, idUsuario).Scan(&grupos)
	return grupos
}

// Labs: cuántos labs hay corriendo y cuántos creándose
func (r *InicioRepository) Labs() (corriendo, creando int) {
	var filas []struct {
		Estado string `gorm:"column:ESTADO"`
		N      int    `gorm:"column:N"`
	}
	r.db.Raw(`SELECT ESTADO, COUNT(*) N FROM LABORATORIOS GROUP BY ESTADO`).Scan(&filas)
	for _, f := range filas {
		switch strings.TrimSpace(f.Estado) {
		case "CORRIENDO":
			corriendo = f.N
		case "CREANDO":
			creando = f.N
		}
	}
	return corriendo, creando
}

func (r *InicioRepository) UsuariosPorTipo() []domain.UsuariosPorTipo {
	lista := []domain.UsuariosPorTipo{}
	r.db.Raw(`
		SELECT T.CLAVE TIPO, T.DESCRIPCION, COUNT(U.ID) TOTAL,
		       COALESCE(SUM(CASE WHEN U.ACTIVO THEN 1 ELSE 0 END), 0) ACTIVOS
		FROM TIPOS_USUARIO T
		LEFT JOIN USUARIOS U ON U.TIPO = T.CLAVE
		GROUP BY T.CLAVE, T.DESCRIPCION
		ORDER BY 3 DESC`).Scan(&lista)
	for i := range lista {
		lista[i].Tipo = strings.TrimSpace(lista[i].Tipo)
	}
	return lista
}

// UltimaImportacion: la más reciente (nil si nunca se ha importado)
func (r *InicioRepository) UltimaImportacion() *domain.ResumenImportacion {
	var i domain.ResumenImportacion
	r.db.Raw(`
		SELECT FIRST 1 I.ARCHIVO, I.TIPO, I.ESTADO, I.INSERTADOS, I.ACTUALIZADOS, I.CON_ERROR, I.INICIO,
		       COALESCE(U.NOMBRE, '') USUARIO
		FROM IMPORTACIONES I
		LEFT JOIN USUARIOS U ON U.ID = I.ID_USUARIO
		ORDER BY I.INICIO DESC`).Scan(&i)
	if i.Archivo == "" {
		return nil
	}
	i.Tipo, i.Estado = strings.TrimSpace(i.Tipo), strings.TrimSpace(i.Estado)
	return &i
}

func (r *InicioRepository) PracticasActivas() int {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM PRACTICAS WHERE ESTADO = 'ACTIVA'`).Scan(&n)
	return n
}
