-- =====================================================================
--  Migración 001: tablas de despliegues de topologías (Plantillas / IA)
--
--  esquema.sql solo se aplica cuando se crea la base. Para una base que
--  ya existe (la VM, la laptop), raccoon.sh corre este archivo con isql.
--  Es idempotente: si las tablas ya existen no hace nada.
--  (Firebird 5 no tiene CREATE TABLE IF NOT EXISTS: se revisa RDB$RELATIONS.)
-- =====================================================================
SET TERM ^ ;

EXECUTE BLOCK AS
BEGIN
    IF (NOT EXISTS (SELECT 1 FROM RDB$RELATIONS WHERE RDB$RELATION_NAME = 'ESTADOS_DESPLIEGUE')) THEN
    BEGIN
        EXECUTE STATEMENT '
            CREATE TABLE ESTADOS_DESPLIEGUE (
                CLAVE        VARCHAR(12)  NOT NULL PRIMARY KEY,
                DESCRIPCION  VARCHAR(40)  NOT NULL
            )';
    END

    IF (NOT EXISTS (SELECT 1 FROM RDB$RELATIONS WHERE RDB$RELATION_NAME = 'DESPLIEGUES')) THEN
    BEGIN
        EXECUTE STATEMENT '
            CREATE TABLE DESPLIEGUES (
                ID              CHAR(16) CHARACTER SET OCTETS  NOT NULL PRIMARY KEY,
                ID_LABORATORIO  CHAR(16) CHARACTER SET OCTETS  NOT NULL REFERENCES LABORATORIOS (ID) ON DELETE CASCADE,
                LAB_ACTIVO      CHAR(16) CHARACTER SET OCTETS  UNIQUE,
                ID_USUARIO      CHAR(16) CHARACTER SET OCTETS  REFERENCES USUARIOS (ID) ON DELETE SET NULL,
                ORIGEN          VARCHAR(20)   NOT NULL,
                PROYECTO        VARCHAR(40)   NOT NULL,
                ESTADO          VARCHAR(12)   DEFAULT ''PROCESANDO'' NOT NULL REFERENCES ESTADOS_DESPLIEGUE (CLAVE),
                PORCENTAJE      SMALLINT      DEFAULT 0 NOT NULL,
                PASO            VARCHAR(120),
                MENSAJE         VARCHAR(1000),
                INICIO          TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
                ACTUALIZADO     TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
                FIN             TIMESTAMP
            )';
    END
END^

SET TERM ; ^
COMMIT;

-- Catálogo (en otra transacción: la tabla debe existir ya)
UPDATE OR INSERT INTO ESTADOS_DESPLIEGUE VALUES ('PROCESANDO', 'Procesando') MATCHING (CLAVE);
UPDATE OR INSERT INTO ESTADOS_DESPLIEGUE VALUES ('TERMINADO',  'Terminado')  MATCHING (CLAVE);
UPDATE OR INSERT INTO ESTADOS_DESPLIEGUE VALUES ('ERROR',      'Error')      MATCHING (CLAVE);
COMMIT;
