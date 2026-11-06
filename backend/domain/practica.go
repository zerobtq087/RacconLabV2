package domain

import "time"

// ---------------------------------------------------------------------
// Prácticas con calificación (igual que models/Practica.ts)
// ---------------------------------------------------------------------

// Practica: lo que ve el profesor en su lista y en el detalle
type Practica struct {
	ID             string         `gorm:"column:ID" json:"id"`
	IDProfesor     string         `gorm:"column:ID_PROFESOR" json:"-"`
	Profesor       string         `gorm:"column:PROFESOR" json:"profesor"`
	IDGrupo        string         `gorm:"column:ID_GRUPO" json:"id_grupo"`
	Grupo          string         `gorm:"column:GRUPO" json:"grupo"`
	Nombre         string         `gorm:"column:NOMBRE" json:"nombre"`
	Instrucciones  string         `gorm:"column:INSTRUCCIONES" json:"instrucciones"`
	EnablePassword string         `gorm:"column:ENABLE_PASSWORD" json:"enable_password"`
	ProyectoOrigen string         `gorm:"column:PROYECTO_ORIGEN" json:"proyecto_origen"`
	Estado         string         `gorm:"column:ESTADO" json:"estado"` // EXPORTANDO, BORRADOR, ACTIVA, CERRADA, ERROR
	Paso           string         `gorm:"column:PASO" json:"paso"`
	Mensaje        string         `gorm:"column:MENSAJE" json:"mensaje"`
	Creada         time.Time      `gorm:"column:CREADA" json:"creada"`
	Activada       *time.Time     `gorm:"column:ACTIVADA" json:"activada"`
	Cerrada        *time.Time     `gorm:"column:CERRADA" json:"cerrada"`
	Entregadas     int            `gorm:"column:ENTREGADAS" json:"entregadas"`
	Alumnos        int            `gorm:"column:ALUMNOS" json:"alumnos"`
	TieneEntregas  bool           `gorm:"-" json:"tiene_entregas"` // criterios bloqueados
	Nodos          []NodoPractica `gorm:"-" json:"nodos"`          // para el formulario de criterios
	Criterios      []Criterio     `gorm:"-" json:"criterios"`
}

// NodoPractica: un nodo del proyecto base con sus puertos (para elegir en los criterios)
type NodoPractica struct {
	Nombre  string   `gorm:"column:NODO" json:"nombre"`
	Tipo    string   `gorm:"column:TIPO_NODO" json:"tipo"` // dynamips, docker, vpcs, ethernet_switch...
	Puertos []string `gorm:"-" json:"puertos"`
}

// Criterio: una revisión que se hace en el lab del alumno al entregar
type Criterio struct {
	ID       string `gorm:"column:ID" json:"id"`
	Orden    int    `gorm:"column:ORDEN" json:"orden"`
	Tipo     string `gorm:"column:TIPO" json:"tipo"` // PING, IP_INTERFAZ, PROTOCOLO, VECINOS_OSPF, RUTA, VLAN_PUERTO, TRUNK, LINEA_CONFIG, DHCP
	Peso     int    `gorm:"column:PESO" json:"peso"`
	Nodo     string `gorm:"column:NODO" json:"nodo"`
	Interfaz string `gorm:"column:INTERFAZ" json:"interfaz"`
	Destino  string `gorm:"column:DESTINO" json:"destino"` // PING: nodo o IP destino
	Valor    string `gorm:"column:VALOR" json:"valor"`     // IP/red, línea, número de vecinos, VLAN...
	Modo     string `gorm:"column:MODO" json:"modo"`       // EXACTA, RED, DEBE, NO_DEBE, OSPF, RIP, ESTATICA, EMPIEZA, CONTIENE
	Mensaje  string `gorm:"column:MENSAJE" json:"mensaje"` // para el alumno (sin la respuesta)
}

// SolicitudPractica: crear o editar (el proyecto solo se indica al crear)
type SolicitudPractica struct {
	Proyecto       string `json:"proyecto"` // ID del proyecto en el GNS3 del profesor
	Nombre         string `json:"nombre"`
	IDGrupo        string `json:"id_grupo"`
	Instrucciones  string `json:"instrucciones"`
	EnablePassword string `json:"enable_password"`
}

// ProyectoGNS3: un proyecto de un lab (para elegir o supervisar)
type ProyectoGNS3 struct {
	ID         string `json:"project_id"`
	Nombre     string `json:"name"`
	Estado     string `json:"status"` // opened | closed
	Nodos      int    `json:"nodos"`
	Encendidos int    `json:"encendidos"`
}

// Revision: avance y resultado de una revisión (entrega o prueba del profesor)
type Revision struct {
	ID           string    `gorm:"column:ID" json:"id"`
	IDPractica   string    `gorm:"column:ID_PRACTICA" json:"-"`
	IDUsuario    string    `gorm:"column:ID_USUARIO" json:"-"`
	Lab          string    `gorm:"column:LAB_ACTIVO" json:"-"`
	EsPrueba     bool      `gorm:"column:ES_PRUEBA" json:"es_prueba"`
	Estado       string    `gorm:"column:ESTADO" json:"estado"` // EN_COLA, REVISANDO, CALIFICADA, ERROR
	Porcentaje   int       `gorm:"column:PORCENTAJE" json:"porcentaje"`
	Paso         string    `gorm:"column:PASO" json:"paso"`
	Mensaje      string    `gorm:"column:MENSAJE" json:"mensaje"`
	Calificacion *int      `gorm:"column:CALIFICACION" json:"calificacion"`
	Inicio       time.Time `gorm:"column:INICIO" json:"inicio"`
	Entrego      string    `gorm:"column:ENTREGO" json:"entrego"` // nombre de quien pulsó "Entregar"
}

// Resultado: un criterio revisado. "Obtenido" solo lo ve el docente.
type Resultado struct {
	IDCriterio  string `gorm:"column:ID_CRITERIO" json:"id_criterio"`
	Tipo        string `gorm:"column:TIPO" json:"tipo"`
	Nodo        string `gorm:"column:NODO" json:"nodo"`
	Peso        int    `gorm:"column:PESO" json:"peso"`
	Cumple      bool   `gorm:"column:CUMPLE" json:"cumple"`
	Puntos      int    `gorm:"column:PUNTOS" json:"puntos"`
	Mensaje     string `gorm:"column:MENSAJE" json:"mensaje"`
	Obtenido    string `gorm:"column:OBTENIDO" json:"obtenido,omitempty"`
	Descripcion string `gorm:"-" json:"descripcion,omitempty"` // qué se pedía (solo docente)
}

// ConfigNodo: running-config de un router guardada al entregar
type ConfigNodo struct {
	Nodo   string `gorm:"column:NODO" json:"nodo"`
	Config string `gorm:"column:CONFIG" json:"config"`
}

// FilaEntrega: un alumno en la tabla del grupo (y en el Excel)
type FilaEntrega struct {
	IDUsuario    string     `gorm:"column:ID_USUARIO" json:"id_usuario"`
	Matricula    string     `gorm:"column:MATRICULA" json:"matricula"`
	Nombre       string     `gorm:"column:NOMBRE" json:"nombre"`
	IDRevision   string     `gorm:"column:ID_REVISION" json:"id_revision"`
	Entregada    *time.Time `gorm:"column:ENTREGADA" json:"entregada"`
	Calificacion *int       `gorm:"column:CALIFICACION" json:"calificacion"`
	Entrego      string     `gorm:"column:ENTREGO" json:"entrego"`
	Estado       string     `gorm:"-" json:"estado"`  // A_TIEMPO, TARDE, PENDIENTE
	Retraso      string     `gorm:"-" json:"retraso"` // "2 h 15 min" (solo tarde)
}

// DetalleEntrega: lo que ve el docente al abrir la entrega de un alumno
type DetalleEntrega struct {
	Fila        FilaEntrega  `json:"fila"`
	Integrantes []Persona    `json:"integrantes"` // a quiénes contó (equipo)
	Resultados  []Resultado  `json:"resultados"`
	Configs     []ConfigNodo `json:"configs"`
	ConProyecto bool         `json:"con_proyecto"`
}

// Prueba: la última revisión de prueba del profesor contra su propia solución
type Prueba struct {
	Revision   *Revision   `json:"revision"`
	Resultados []Resultado `json:"resultados"`
}

// MiPractica: lo que ve el alumno (después de entregar, solo su resultado)
type MiPractica struct {
	ID            string     `gorm:"column:ID" json:"id"`
	Nombre        string     `gorm:"column:NOMBRE" json:"nombre"`
	Instrucciones string     `gorm:"column:INSTRUCCIONES" json:"instrucciones,omitempty"`
	Estado        string     `gorm:"column:ESTADO" json:"estado"` // ACTIVA o CERRADA
	Cerrada       *time.Time `gorm:"column:CERRADA" json:"cerrada"`
	Profesor      string     `gorm:"column:PROFESOR" json:"profesor"`
	IDRevision    string     `gorm:"column:ID_REVISION" json:"-"`
	Entregada     *time.Time `gorm:"column:ENTREGADA" json:"entregada"`
	Calificacion  *int       `gorm:"column:CALIFICACION" json:"calificacion"`
	Tarde         bool       `gorm:"-" json:"tarde"`
	Retraso       string     `gorm:"-" json:"retraso"`
	Fallidos      []string   `gorm:"-" json:"fallidos"` // mensajes de los criterios que no cumplió
	Revision      *Revision  `gorm:"-" json:"revision"` // revisión en curso en su lab
}

// SolicitudAbrir: abrir la práctica en el lab del alumno
type SolicitudAbrir struct {
	DetenerOtros   bool `json:"detener_otros"`    // detener y cerrar los otros proyectos encendidos
	EmpezarDeNuevo bool `json:"empezar_de_nuevo"` // borrar la copia y volver a importar
}

// ---------------------------------------------------------------------
// Supervisión de labs (docente y admin)
// ---------------------------------------------------------------------

// AlumnoLab: un alumno del grupo y su lab (sin credenciales)
type AlumnoLab struct {
	IDUsuario string `gorm:"column:ID_USUARIO" json:"id_usuario"`
	Matricula string `gorm:"column:MATRICULA" json:"matricula"`
	Nombre    string `gorm:"column:NOMBRE" json:"nombre"`
	IDLab     string `gorm:"column:ID_LAB" json:"id_lab"`
	Puerto    int    `gorm:"column:PUERTO_BASE" json:"puerto"`
	Estado    string `gorm:"column:ESTADO" json:"estado"`
	EsDueno   bool   `gorm:"column:ES_DUENO" json:"es_dueno"`
}

// NodoSupervision: un nodo con su configuración (guardada o actual)
type NodoSupervision struct {
	Nombre  string `json:"nombre"`
	Tipo    string `json:"tipo"`
	Estado  string `json:"estado"`
	Consola int    `json:"consola"`
	Config  string `json:"config"`
	Error   string `json:"error,omitempty"`
}

// EntradaBitacora: una consulta de un docente al lab de un alumno
type EntradaBitacora struct {
	Fecha   time.Time `gorm:"column:FECHA" json:"fecha"`
	Docente string    `gorm:"column:DOCENTE" json:"docente"`
	Alumno  string    `gorm:"column:ALUMNO" json:"alumno"`
	Accion  string    `gorm:"column:ACCION" json:"accion"`
	Detalle string    `gorm:"column:DETALLE" json:"detalle"`
}

// Acceso: una IP que puede usar el bloque de puertos de un lab (para el firewall)
type Acceso struct {
	IP     string `gorm:"column:IP"`
	Puerto int    `gorm:"column:PUERTO_BASE"`
}
