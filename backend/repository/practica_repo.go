package repository

import (
	"errors"
	"strings"

	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrRevisionEnCurso = errors.New("ya se está revisando una entrega en este laboratorio; espera a que termine")
	ErrPracticaNoEsta  = errors.New("la práctica no existe")
)

type PracticaRepository struct {
	db *gorm.DB
}

func NewPracticaRepository(db *gorm.DB) *PracticaRepository {
	return &PracticaRepository{db: db}
}

const columnasPractica = `
	SELECT UUID_TO_CHAR(P.ID) ID, UUID_TO_CHAR(P.ID_PROFESOR) ID_PROFESOR, COALESCE(U.NOMBRE, '') PROFESOR,
	       UUID_TO_CHAR(P.ID_GRUPO) ID_GRUPO, G.CODIGO GRUPO, P.NOMBRE,
	       COALESCE(P.INSTRUCCIONES, '') INSTRUCCIONES,
	       COALESCE(P.PROYECTO_ORIGEN, '') PROYECTO_ORIGEN, P.ESTADO, COALESCE(P.PASO, '') PASO,
	       COALESCE(P.MENSAJE, '') MENSAJE, P.CREADA, P.ACTIVADA, P.CERRADA,
	       (SELECT COUNT(*) FROM ENTREGAS E WHERE E.ID_PRACTICA = P.ID) ENTREGADAS,
	       (SELECT COUNT(*) FROM USUARIOS A WHERE A.ID_GRUPO = P.ID_GRUPO AND A.ACTIVO) ALUMNOS
	FROM PRACTICAS P
	JOIN GRUPOS G ON G.ID = P.ID_GRUPO
	LEFT JOIN USUARIOS U ON U.ID = P.ID_PROFESOR`

// ---------------------------------------------------------------------
// Permisos
// ---------------------------------------------------------------------

// EsAdmin: el usuario tiene el rol ADMIN (puede crear prácticas para cualquier grupo)
func (r *PracticaRepository) EsAdmin(idUsuario string) bool {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM USUARIO_ROLES WHERE ID_USUARIO = CHAR_TO_UUID(?) AND ROL = 'ADMIN'`, idUsuario).Scan(&n)
	return n > 0
}

// EsDocente: el usuario es docente del grupo (GRUPO_DOCENTES)
func (r *PracticaRepository) EsDocente(idUsuario, idGrupo string) bool {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM GRUPO_DOCENTES WHERE ID_USUARIO = CHAR_TO_UUID(?) AND ID_GRUPO = CHAR_TO_UUID(?)`,
		idUsuario, idGrupo).Scan(&n)
	return n > 0
}

// GrupoDe: el grupo del alumno ("" si no tiene). Ojo: COALESCE con un CHAR(36)
// rellena ” con espacios; por eso los UUID opcionales se leen con TRIM.
func (r *PracticaRepository) GrupoDe(idUsuario string) string {
	var g string
	r.db.Raw(`SELECT TRIM(COALESCE(UUID_TO_CHAR(ID_GRUPO), '')) FROM USUARIOS WHERE ID = CHAR_TO_UUID(?)`, idUsuario).Scan(&g)
	return g
}

// ---------------------------------------------------------------------
// Prácticas
// ---------------------------------------------------------------------

// Crear registra la práctica en EXPORTANDO (el proyecto se exporta en segundo plano)
func (r *PracticaRepository) Crear(idProfesor string, s *domain.SolicitudPractica, origen string) (string, error) {
	id := uuid.NewString()
	err := r.db.Exec(`
		INSERT INTO PRACTICAS (ID, ID_PROFESOR, ID_GRUPO, NOMBRE, INSTRUCCIONES, PROYECTO_ORIGEN, PASO)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?, ?, 'Preparando')`,
		id, idProfesor, s.IDGrupo, s.Nombre, s.Instrucciones, origen).Error
	return id, err
}

func (r *PracticaRepository) Paso(id, paso string) {
	r.db.Exec(`UPDATE PRACTICAS SET PASO = ?, ACTUALIZADO = CURRENT_TIMESTAMP WHERE ID = CHAR_TO_UUID(?)`, paso, id)
}

func (r *PracticaRepository) FallaExportar(id, mensaje string) {
	if len(mensaje) > 500 {
		mensaje = mensaje[:500]
	}
	r.db.Exec(`UPDATE PRACTICAS SET ESTADO = 'ERROR', MENSAJE = ?, ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`, mensaje, id)
}

// Exportada guarda el proyecto base y sus nodos/puertos; la práctica queda en BORRADOR
func (r *PracticaRepository) Exportada(id string, zip []byte, nodos []domain.NodoPractica) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO PRACTICA_PROYECTOS (ID_PRACTICA, ARCHIVO, TAMANO) VALUES (CHAR_TO_UUID(?), ?, ?)`,
			id, zip, len(zip)).Error; err != nil {
			return err
		}
		for _, n := range nodos {
			if err := tx.Exec(`INSERT INTO PRACTICA_NODOS (ID_PRACTICA, NODO, TIPO_NODO) VALUES (CHAR_TO_UUID(?), ?, ?)`,
				id, n.Nombre, n.Tipo).Error; err != nil {
				return err
			}
			for k, p := range n.Puertos {
				if err := tx.Exec(`INSERT INTO PRACTICA_PUERTOS (ID_PRACTICA, NODO, PUERTO, ORDEN) VALUES (CHAR_TO_UUID(?), ?, ?, ?)`,
					id, n.Nombre, p, k).Error; err != nil {
					return err
				}
			}
		}
		return tx.Exec(`UPDATE PRACTICAS SET ESTADO = 'BORRADOR', PASO = '', MENSAJE = '', ACTUALIZADO = CURRENT_TIMESTAMP
			WHERE ID = CHAR_TO_UUID(?)`, id).Error
	})
}

// Listar: las prácticas del profesor (el admin ve todas)
func (r *PracticaRepository) Listar(idUsuario string, todas bool) []domain.Practica {
	// Si una réplica murió exportando, la práctica queda en ERROR
	r.db.Exec(`UPDATE PRACTICAS SET ESTADO = 'ERROR', MENSAJE = 'Se interrumpió la exportación; vuelve a crearla'
		WHERE ESTADO = 'EXPORTANDO' AND ACTUALIZADO < DATEADD(-5 MINUTE TO CURRENT_TIMESTAMP)`)

	practicas := []domain.Practica{}
	if todas {
		r.db.Raw(columnasPractica + ` ORDER BY P.CREADA DESC`).Scan(&practicas)
	} else {
		r.db.Raw(columnasPractica+` WHERE P.ID_PROFESOR = CHAR_TO_UUID(?) ORDER BY P.CREADA DESC`, idUsuario).Scan(&practicas)
	}
	return practicas
}

// PorID: la práctica con sus nodos y criterios (nil si no existe)
func (r *PracticaRepository) PorID(id string) *domain.Practica {
	var p domain.Practica
	r.db.Raw(columnasPractica+` WHERE P.ID = CHAR_TO_UUID(?)`, id).Scan(&p)
	if p.ID == "" {
		return nil
	}
	p.Nodos = r.Nodos(id)
	p.Criterios = r.Criterios(id)
	p.TieneEntregas = p.Entregadas > 0
	return &p
}

func (r *PracticaRepository) Nodos(id string) []domain.NodoPractica {
	nodos := []domain.NodoPractica{}
	r.db.Raw(`SELECT NODO, TIPO_NODO FROM PRACTICA_NODOS WHERE ID_PRACTICA = CHAR_TO_UUID(?) ORDER BY NODO`, id).Scan(&nodos)
	for i := range nodos {
		nodos[i].Puertos = []string{}
		r.db.Raw(`SELECT PUERTO FROM PRACTICA_PUERTOS WHERE ID_PRACTICA = CHAR_TO_UUID(?) AND NODO = ? ORDER BY ORDEN`,
			id, nodos[i].Nombre).Scan(&nodos[i].Puertos)
	}
	return nodos
}

func (r *PracticaRepository) Criterios(id string) []domain.Criterio {
	criterios := []domain.Criterio{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(ID) ID, ORDEN, TIPO, PESO, NODO, COALESCE(INTERFAZ, '') INTERFAZ,
		       COALESCE(DESTINO, '') DESTINO, COALESCE(VALOR, '') VALOR, COALESCE(MODO, '') MODO,
		       COALESCE(MENSAJE, '') MENSAJE
		FROM CRITERIOS WHERE ID_PRACTICA = CHAR_TO_UUID(?) ORDER BY ORDEN`, id).Scan(&criterios)
	for i := range criterios {
		criterios[i].Tipo = strings.TrimSpace(criterios[i].Tipo)
	}
	return criterios
}

func (r *PracticaRepository) Actualizar(id string, s *domain.SolicitudPractica) error {
	return r.db.Exec(`
		UPDATE PRACTICAS SET NOMBRE = ?, ID_GRUPO = CHAR_TO_UUID(?), INSTRUCCIONES = ?
		WHERE ID = CHAR_TO_UUID(?)`, s.Nombre, s.IDGrupo, s.Instrucciones, id).Error
}

// GuardarCriterios reemplaza todos los criterios (el servicio revisa que no haya entregas)
func (r *PracticaRepository) GuardarCriterios(id string, criterios []domain.Criterio) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM CRITERIOS WHERE ID_PRACTICA = CHAR_TO_UUID(?)`, id).Error; err != nil {
			return err
		}
		for i, c := range criterios {
			if err := tx.Exec(`
				INSERT INTO CRITERIOS (ID, ID_PRACTICA, ORDEN, TIPO, PESO, NODO, INTERFAZ, DESTINO, VALOR, MODO, MENSAJE)
				VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid.NewString(), id, i+1, c.Tipo, c.Peso, c.Nodo, c.Interfaz, c.Destino, c.Valor, c.Modo, c.Mensaje).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PracticaRepository) Activar(id string) error {
	return r.db.Exec(`UPDATE PRACTICAS SET ESTADO = 'ACTIVA', ACTIVADA = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?) AND ESTADO = 'BORRADOR'`, id).Error
}

// Cerrar guarda la fecha y hora del cierre (no se reabre)
func (r *PracticaRepository) Cerrar(id string) error {
	return r.db.Exec(`UPDATE PRACTICAS SET ESTADO = 'CERRADA', CERRADA = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?) AND ESTADO = 'ACTIVA'`, id).Error
}

func (r *PracticaRepository) Borrar(id string) error {
	return r.db.Exec(`DELETE FROM PRACTICAS WHERE ID = CHAR_TO_UUID(?)`, id).Error
}

// archivo: un BLOB binario (con Scan directo a []byte GORM solo deja 1 byte:
// por eso se lee en una estructura)
type archivo struct {
	Archivo []byte `gorm:"column:ARCHIVO"`
}

// ProyectoBase: el .gns3project de la práctica
func (r *PracticaRepository) ProyectoBase(id string) []byte {
	var a archivo
	r.db.Raw(`SELECT ARCHIVO FROM PRACTICA_PROYECTOS WHERE ID_PRACTICA = CHAR_TO_UUID(?)`, id).Scan(&a)
	return a.Archivo
}

// ---------------------------------------------------------------------
// Revisiones (entregas y pruebas del profesor)
// ---------------------------------------------------------------------

const columnasRevision = `
	SELECT UUID_TO_CHAR(R.ID) ID, UUID_TO_CHAR(R.ID_PRACTICA) ID_PRACTICA,
	       TRIM(COALESCE(UUID_TO_CHAR(R.ID_USUARIO), '')) ID_USUARIO, TRIM(COALESCE(UUID_TO_CHAR(R.LAB_ACTIVO), '')) LAB_ACTIVO,
	       R.ES_PRUEBA, R.ESTADO, R.PORCENTAJE, COALESCE(R.PASO, '') PASO, COALESCE(R.MENSAJE, '') MENSAJE,
	       R.CALIFICACION, R.INICIO, COALESCE(U.NOMBRE, '') ENTREGO
	FROM REVISIONES R
	LEFT JOIN USUARIOS U ON U.ID = R.ID_USUARIO`

// limpiarRevisiones: si una réplica murió a medio revisar, su latido se detiene
func (r *PracticaRepository) limpiarRevisiones() {
	r.db.Exec(`UPDATE REVISIONES SET ESTADO = 'ERROR', LAB_ACTIVO = NULL, FIN = CURRENT_TIMESTAMP,
		       MENSAJE = 'Se interrumpió la revisión (el servidor se reinició); vuelve a entregar'
		WHERE ESTADO IN ('REVISANDO', 'EN_COLA') AND ACTUALIZADO < DATEADD(-5 MINUTE TO CURRENT_TIMESTAMP)`)
}

// ReservarRevision registra la revisión. LAB_ACTIVO es UNIQUE: una sola a la vez por lab.
func (r *PracticaRepository) ReservarRevision(idPractica, idUsuario, idLab string, esPrueba, enCola bool) (string, error) {
	r.limpiarRevisiones()
	estado := "REVISANDO"
	if enCola {
		estado = "EN_COLA"
	}
	id := uuid.NewString()
	err := r.db.Exec(`
		INSERT INTO REVISIONES (ID, ID_PRACTICA, ID_USUARIO, LAB_ACTIVO, ES_PRUEBA, ESTADO, PASO)
		VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?, 'En espera')`,
		id, idPractica, idUsuario, idLab, esPrueba, estado).Error
	if err != nil {
		var n int
		r.db.Raw(`SELECT COUNT(*) FROM REVISIONES WHERE LAB_ACTIVO = CHAR_TO_UUID(?)`, idLab).Scan(&n)
		if n > 0 {
			return "", ErrRevisionEnCurso
		}
		return "", err
	}
	return id, nil
}

func (r *PracticaRepository) Revision(id string) *domain.Revision {
	var rev domain.Revision
	r.db.Raw(columnasRevision+` WHERE R.ID = CHAR_TO_UUID(?)`, id).Scan(&rev)
	if rev.ID == "" {
		return nil
	}
	return &rev
}

// RevisionEnCurso: la revisión que se está haciendo en ese lab para esa práctica (nil si no hay)
func (r *PracticaRepository) RevisionEnCurso(idLab, idPractica string) *domain.Revision {
	r.limpiarRevisiones()
	var rev domain.Revision
	r.db.Raw(columnasRevision+` WHERE R.LAB_ACTIVO = CHAR_TO_UUID(?) AND R.ID_PRACTICA = CHAR_TO_UUID(?)`,
		idLab, idPractica).Scan(&rev)
	if rev.ID == "" {
		return nil
	}
	return &rev
}

// RevisandoEnLab: si el lab tiene una revisión en curso (de cualquier práctica)
func (r *PracticaRepository) RevisandoEnLab(idLab string) bool {
	r.limpiarRevisiones()
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM REVISIONES WHERE LAB_ACTIVO = CHAR_TO_UUID(?)`, idLab).Scan(&n)
	return n > 0
}

// UltimaRevision: la última revisión (entrega o prueba) que pidió este usuario de esta práctica
func (r *PracticaRepository) UltimaRevision(idPractica, idUsuario string, esPrueba bool) *domain.Revision {
	r.limpiarRevisiones()
	var rev domain.Revision
	r.db.Raw(`SELECT FIRST 1 `+strings.TrimPrefix(strings.TrimSpace(columnasRevision), "SELECT")+`
		WHERE R.ID_PRACTICA = CHAR_TO_UUID(?) AND R.ID_USUARIO = CHAR_TO_UUID(?) AND R.ES_PRUEBA = ?
		ORDER BY R.INICIO DESC`, idPractica, idUsuario, esPrueba).Scan(&rev)
	if rev.ID == "" {
		return nil
	}
	return &rev
}

// TomarDeCola: una réplica toma la revisión en cola más vieja (si otra la tomó primero, no pasa nada)
func (r *PracticaRepository) TomarDeCola() string {
	var id string
	r.db.Raw(`SELECT FIRST 1 UUID_TO_CHAR(ID) FROM REVISIONES WHERE ESTADO = 'EN_COLA' ORDER BY INICIO`).Scan(&id)
	if id == "" {
		return ""
	}
	res := r.db.Exec(`UPDATE REVISIONES SET ESTADO = 'REVISANDO', ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?) AND ESTADO = 'EN_COLA'`, id)
	if res.RowsAffected != 1 {
		return ""
	}
	return id
}

func (r *PracticaRepository) Revisando() int {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM REVISIONES WHERE ESTADO = 'REVISANDO'`).Scan(&n)
	return n
}

// Avance guarda el porcentaje y el paso, y renueva el latido
func (r *PracticaRepository) Avance(id string, porcentaje int, paso string) {
	if len(paso) > 120 {
		paso = paso[:120]
	}
	r.db.Exec(`UPDATE REVISIONES SET PORCENTAJE = ?, PASO = ?, ACTUALIZADO = CURRENT_TIMESTAMP
		WHERE ID = CHAR_TO_UUID(?)`, porcentaje, paso, id)
}

func (r *PracticaRepository) Latido(id string) {
	r.db.Exec(`UPDATE REVISIONES SET ACTUALIZADO = CURRENT_TIMESTAMP WHERE ID = CHAR_TO_UUID(?)`, id)
}

func (r *PracticaRepository) GuardarResultados(id string, resultados []domain.Resultado) {
	for _, x := range resultados {
		obtenido := x.Obtenido
		if len(obtenido) > 500 {
			obtenido = obtenido[:500]
		}
		r.db.Exec(`
			UPDATE OR INSERT INTO REVISION_RESULTADOS (ID_REVISION, ID_CRITERIO, CUMPLE, PUNTOS, OBTENIDO)
			VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), ?, ?, ?) MATCHING (ID_REVISION, ID_CRITERIO)`,
			id, x.IDCriterio, x.Cumple, x.Puntos, obtenido)
	}
}

func (r *PracticaRepository) GuardarConfigs(id string, configs []domain.ConfigNodo) {
	for _, c := range configs {
		r.db.Exec(`UPDATE OR INSERT INTO REVISION_CONFIGS (ID_REVISION, NODO, CONFIG)
			VALUES (CHAR_TO_UUID(?), ?, ?) MATCHING (ID_REVISION, NODO)`, id, c.Nodo, c.Config)
	}
}

func (r *PracticaRepository) GuardarProyectoRevision(id string, zip []byte) error {
	return r.db.Exec(`INSERT INTO REVISION_PROYECTO (ID_REVISION, ARCHIVO, TAMANO) VALUES (CHAR_TO_UUID(?), ?, ?)`,
		id, zip, len(zip)).Error
}

// TerminarRevision cierra la revisión y libera el lab
func (r *PracticaRepository) TerminarRevision(id, estado string, calificacion *int, mensaje string) {
	if len(mensaje) > 500 {
		mensaje = mensaje[:500]
	}
	r.db.Exec(`UPDATE REVISIONES SET ESTADO = ?, CALIFICACION = ?, MENSAJE = ?, LAB_ACTIVO = NULL,
		       FIN = CURRENT_TIMESTAMP, ACTUALIZADO = CURRENT_TIMESTAMP,
		       PORCENTAJE = IIF(CAST(? AS VARCHAR(12)) = 'CALIFICADA', 100, PORCENTAJE)
		WHERE ID = CHAR_TO_UUID(?)`, estado, calificacion, mensaje, estado, id)
}

// RegistrarEntregas: la revisión cuenta como entrega para estos alumnos.
// La PK (práctica, alumno) impide una segunda entrega: los que ya tenían, se saltan.
func (r *PracticaRepository) RegistrarEntregas(idPractica, idRevision string, usuarios []string) int {
	n := 0
	for _, u := range usuarios {
		if r.db.Exec(`INSERT INTO ENTREGAS (ID_PRACTICA, ID_USUARIO, ID_REVISION)
			VALUES (CHAR_TO_UUID(?), CHAR_TO_UUID(?), CHAR_TO_UUID(?))`, idPractica, u, idRevision).Error == nil {
			n++
		}
	}
	return n
}

// YaEntrego: el alumno ya tiene su entrega de esa práctica
func (r *PracticaRepository) YaEntrego(idPractica, idUsuario string) bool {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM ENTREGAS WHERE ID_PRACTICA = CHAR_TO_UUID(?) AND ID_USUARIO = CHAR_TO_UUID(?)`,
		idPractica, idUsuario).Scan(&n)
	return n > 0
}

// ---------------------------------------------------------------------
// Entregas (docente y alumno)
// ---------------------------------------------------------------------

// TablaGrupo: todos los alumnos del grupo con su entrega (o sin ella)
func (r *PracticaRepository) TablaGrupo(idPractica string) []domain.FilaEntrega {
	filas := []domain.FilaEntrega{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(A.ID) ID_USUARIO, A.MATRICULA, A.NOMBRE,
		       TRIM(COALESCE(UUID_TO_CHAR(E.ID_REVISION), '')) ID_REVISION, R.INICIO ENTREGADA, R.CALIFICACION,
		       COALESCE(Q.NOMBRE, '') ENTREGO
		FROM PRACTICAS P
		JOIN USUARIOS A ON A.ID_GRUPO = P.ID_GRUPO AND A.ACTIVO
		LEFT JOIN ENTREGAS E ON E.ID_PRACTICA = P.ID AND E.ID_USUARIO = A.ID
		LEFT JOIN REVISIONES R ON R.ID = E.ID_REVISION
		LEFT JOIN USUARIOS Q ON Q.ID = R.ID_USUARIO
		WHERE P.ID = CHAR_TO_UUID(?)
		ORDER BY A.NOMBRE`, idPractica).Scan(&filas)
	return filas
}

// Resultados de una revisión con lo que pedía cada criterio
func (r *PracticaRepository) Resultados(idRevision string) []domain.Resultado {
	res := []domain.Resultado{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(C.ID) ID_CRITERIO, C.TIPO, C.NODO, C.PESO, X.CUMPLE, X.PUNTOS,
		       COALESCE(X.OBTENIDO, '') OBTENIDO, COALESCE(C.MENSAJE, '') MENSAJE
		FROM REVISION_RESULTADOS X
		JOIN CRITERIOS C ON C.ID = X.ID_CRITERIO
		WHERE X.ID_REVISION = CHAR_TO_UUID(?)
		ORDER BY C.ORDEN`, idRevision).Scan(&res)
	for i := range res {
		res[i].Tipo = strings.TrimSpace(res[i].Tipo)
	}
	return res
}

func (r *PracticaRepository) Configs(idRevision string) []domain.ConfigNodo {
	configs := []domain.ConfigNodo{}
	r.db.Raw(`SELECT NODO, COALESCE(CONFIG, '') CONFIG FROM REVISION_CONFIGS
		WHERE ID_REVISION = CHAR_TO_UUID(?) ORDER BY NODO`, idRevision).Scan(&configs)
	return configs
}

// Integrantes: los alumnos a los que contó la revisión
func (r *PracticaRepository) Integrantes(idRevision string) []domain.Persona {
	p := []domain.Persona{}
	r.db.Raw(`SELECT UUID_TO_CHAR(U.ID) ID, U.MATRICULA, U.NOMBRE FROM ENTREGAS E
		JOIN USUARIOS U ON U.ID = E.ID_USUARIO WHERE E.ID_REVISION = CHAR_TO_UUID(?) ORDER BY U.NOMBRE`, idRevision).Scan(&p)
	return p
}

func (r *PracticaRepository) ProyectoRevision(idRevision string) []byte {
	var a archivo
	r.db.Raw(`SELECT ARCHIVO FROM REVISION_PROYECTO WHERE ID_REVISION = CHAR_TO_UUID(?)`, idRevision).Scan(&a)
	return a.Archivo
}

// TieneProyecto: si la entrega guardó la copia del proyecto (sin leer el BLOB)
func (r *PracticaRepository) TieneProyecto(idRevision string) bool {
	var n int
	r.db.Raw(`SELECT COUNT(*) FROM REVISION_PROYECTO WHERE ID_REVISION = CHAR_TO_UUID(?)`, idRevision).Scan(&n)
	return n > 0
}

// MisPracticas: las prácticas activas o cerradas del grupo del alumno, con su entrega
func (r *PracticaRepository) MisPracticas(idUsuario string) []domain.MiPractica {
	lista := []domain.MiPractica{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(P.ID) ID, P.NOMBRE, COALESCE(P.INSTRUCCIONES, '') INSTRUCCIONES, P.ESTADO, P.CERRADA,
		       COALESCE(D.NOMBRE, '') PROFESOR, TRIM(COALESCE(UUID_TO_CHAR(E.ID_REVISION), '')) ID_REVISION,
		       R.INICIO ENTREGADA, R.CALIFICACION
		FROM USUARIOS A
		JOIN PRACTICAS P ON P.ID_GRUPO = A.ID_GRUPO AND P.ESTADO IN ('ACTIVA', 'CERRADA')
		LEFT JOIN USUARIOS D ON D.ID = P.ID_PROFESOR
		LEFT JOIN ENTREGAS E ON E.ID_PRACTICA = P.ID AND E.ID_USUARIO = A.ID
		LEFT JOIN REVISIONES R ON R.ID = E.ID_REVISION
		WHERE A.ID = CHAR_TO_UUID(?)
		ORDER BY P.ACTIVADA DESC`, idUsuario).Scan(&lista)
	return lista
}

// ---------------------------------------------------------------------
// Supervisión
// ---------------------------------------------------------------------

// AlumnosConLab: alumnos del grupo con el lab en el que están (dueño o miembro)
func (r *PracticaRepository) AlumnosConLab(idGrupo string) []domain.AlumnoLab {
	lista := []domain.AlumnoLab{}
	r.db.Raw(`
		SELECT UUID_TO_CHAR(A.ID) ID_USUARIO, A.MATRICULA, A.NOMBRE,
		       TRIM(COALESCE(UUID_TO_CHAR(L.ID), '')) ID_LAB, COALESCE(L.PUERTO_BASE, 0) PUERTO_BASE,
		       COALESCE(L.ESTADO, '') ESTADO, IIF(L.ID_DUENO = A.ID, TRUE, FALSE) ES_DUENO
		FROM USUARIOS A
		LEFT JOIN LABORATORIO_MIEMBROS M ON M.ID_USUARIO = A.ID
		LEFT JOIN LABORATORIOS L ON L.ID_DUENO = A.ID OR L.ID = M.ID_LABORATORIO
		WHERE A.ID_GRUPO = CHAR_TO_UUID(?) AND A.ACTIVO
		ORDER BY A.NOMBRE`, idGrupo).Scan(&lista)
	return lista
}

// GruposDelLab: los grupos de las personas del lab (dueño y equipo), para revisar permisos
func (r *PracticaRepository) GruposDelLab(idLab string) []string {
	grupos := []string{}
	r.db.Raw(`
		SELECT DISTINCT UUID_TO_CHAR(U.ID_GRUPO) FROM USUARIOS U
		WHERE U.ID_GRUPO IS NOT NULL AND (
		      U.ID = (SELECT ID_DUENO FROM LABORATORIOS WHERE ID = CHAR_TO_UUID(?))
		   OR U.ID IN (SELECT ID_USUARIO FROM LABORATORIO_MIEMBROS WHERE ID_LABORATORIO = CHAR_TO_UUID(?)))`,
		idLab, idLab).Scan(&grupos)
	return grupos
}
