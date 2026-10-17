-- =====================================================================
--  Raccoon Lab - Base de datos normalizada (Firebird 5)
--  Igual que el proyecto del profe: llaves UUID con CHAR_TO_UUID/UUID_TO_CHAR
--  La matrícula NO es llave primaria: se puede editar sin romper relaciones
-- =====================================================================

-- ---------------------------------------------------------------------
--  CATÁLOGOS
-- ---------------------------------------------------------------------
CREATE TABLE ROLES (
    CLAVE        VARCHAR(10)  NOT NULL PRIMARY KEY,   -- ADMIN, PROFESOR, ALUMNO
    DESCRIPCION  VARCHAR(40)  NOT NULL,
    NIVEL        SMALLINT     NOT NULL                -- 3, 2, 1
);

CREATE TABLE TIPOS_USUARIO (
    CLAVE        VARCHAR(10)  NOT NULL PRIMARY KEY,   -- alumnos, maestros, admins
    DESCRIPCION  VARCHAR(40)  NOT NULL
);

-- Roles que recibe cada tipo de usuario al importarlo
CREATE TABLE TIPO_ROLES (
    TIPO  VARCHAR(10)  NOT NULL REFERENCES TIPOS_USUARIO (CLAVE),
    ROL   VARCHAR(10)  NOT NULL REFERENCES ROLES (CLAVE),
    PRIMARY KEY (TIPO, ROL)
);

CREATE TABLE CARRERAS (
    ID      CHAR(16) CHARACTER SET OCTETS  NOT NULL PRIMARY KEY,
    NOMBRE  VARCHAR(120)                   NOT NULL UNIQUE
);

-- ---------------------------------------------------------------------
--  GRUPOS Y USUARIOS
-- ---------------------------------------------------------------------
CREATE TABLE GRUPOS (
    ID          CHAR(16) CHARACTER SET OCTETS  NOT NULL PRIMARY KEY,
    CODIGO      VARCHAR(15)                    NOT NULL UNIQUE,   -- IRD-71
    ID_CARRERA  CHAR(16) CHARACTER SET OCTETS  REFERENCES CARRERAS (ID) ON DELETE SET NULL,
    CREADO      TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE USUARIOS (
    ID             CHAR(16) CHARACTER SET OCTETS  NOT NULL PRIMARY KEY,
    MATRICULA      VARCHAR(20)                    NOT NULL UNIQUE,   -- es el login
    NOMBRE         VARCHAR(120)                   NOT NULL,
    PASSWORD       VARCHAR(100)                   NOT NULL,          -- hash bcrypt
    TIPO           VARCHAR(10)                    NOT NULL REFERENCES TIPOS_USUARIO (CLAVE),
    ID_GRUPO       CHAR(16) CHARACTER SET OCTETS  REFERENCES GRUPOS (ID) ON DELETE SET NULL,
    ACTIVO         BOOLEAN   DEFAULT TRUE              NOT NULL,
    CREADO         TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ULTIMO_ACCESO  TIMESTAMP
);

CREATE TABLE USUARIO_ROLES (
    ID_USUARIO  CHAR(16) CHARACTER SET OCTETS  NOT NULL REFERENCES USUARIOS (ID) ON DELETE CASCADE,
    ROL         VARCHAR(10)                    NOT NULL REFERENCES ROLES (CLAVE),
    PRIMARY KEY (ID_USUARIO, ROL)
);

-- Maestros asignados a cada grupo
CREATE TABLE GRUPO_DOCENTES (
    ID_GRUPO    CHAR(16) CHARACTER SET OCTETS  NOT NULL REFERENCES GRUPOS (ID) ON DELETE CASCADE,
    ID_USUARIO  CHAR(16) CHARACTER SET OCTETS  NOT NULL REFERENCES USUARIOS (ID) ON DELETE CASCADE,
    PRIMARY KEY (ID_GRUPO, ID_USUARIO)
);

-- Bloqueo por intentos fallidos del login
CREATE TABLE INTENTOS_LOGIN (
    MATRICULA        VARCHAR(20)  NOT NULL PRIMARY KEY,
    FALLOS           SMALLINT     DEFAULT 0 NOT NULL,
    BLOQUEADO_HASTA  TIMESTAMP
);

-- ---------------------------------------------------------------------
--  DATOS DE LOS CATÁLOGOS
-- ---------------------------------------------------------------------
INSERT INTO ROLES VALUES ('ADMIN',    'Administrador', 3);
INSERT INTO ROLES VALUES ('PROFESOR', 'Maestro',       2);
INSERT INTO ROLES VALUES ('ALUMNO',   'Alumno',        1);

INSERT INTO TIPOS_USUARIO VALUES ('alumnos',  'Alumnos');
INSERT INTO TIPOS_USUARIO VALUES ('maestros', 'Maestros');
INSERT INTO TIPOS_USUARIO VALUES ('admins',   'Administradores');

INSERT INTO TIPO_ROLES VALUES ('alumnos',  'ALUMNO');
INSERT INTO TIPO_ROLES VALUES ('maestros', 'PROFESOR');
INSERT INTO TIPO_ROLES VALUES ('maestros', 'ALUMNO');
INSERT INTO TIPO_ROLES VALUES ('admins',   'ADMIN');
INSERT INTO TIPO_ROLES VALUES ('admins',   'PROFESOR');
INSERT INTO TIPO_ROLES VALUES ('admins',   'ALUMNO');

COMMIT;
