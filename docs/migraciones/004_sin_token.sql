-- =====================================================================
--  Migración 004: GNS3 sin contraseña y prácticas sin "contraseña de enable".
--  Idempotente (raccoon.sh la corre siempre) y segura con datos:
--   - LABORATORIOS.TOKEN: los labs ya creados siguen con el GNS3 que pedía
--     contraseña; antes de quitar la columna se les deja un aviso para que
--     el dueño los limpie y los cree de nuevo (sus proyectos no se pierden:
--     están en gns3_projects).
--   - PRACTICAS.ENABLE_PASSWORD: las contraseñas ahora van en las
--     instrucciones y en los criterios ("enable secret ...").
--   - LATIDOS: cuándo avisó por última vez el agente del firewall.
-- =====================================================================
SET TERM ^ ;

-- 1) Aviso a los labs creados con la versión anterior (antes de quitar TOKEN)
EXECUTE BLOCK AS
BEGIN
    IF (EXISTS (SELECT 1 FROM RDB$RELATION_FIELDS
                WHERE RDB$RELATION_NAME = 'LABORATORIOS' AND RDB$FIELD_NAME = 'TOKEN')) THEN
        EXECUTE STATEMENT '
            UPDATE LABORATORIOS SET MENSAJE = ''Este laboratorio se creó con la versión anterior (GNS3 con contraseña). '' ||
                ''Límpialo y créalo de nuevo; tus proyectos se conservan.''';
END^

SET TERM ; ^
COMMIT;

SET TERM ^ ;

-- 2) Se quitan las columnas que ya no se usan
EXECUTE BLOCK AS
BEGIN
    IF (EXISTS (SELECT 1 FROM RDB$RELATION_FIELDS
                WHERE RDB$RELATION_NAME = 'LABORATORIOS' AND RDB$FIELD_NAME = 'TOKEN')) THEN
        EXECUTE STATEMENT 'ALTER TABLE LABORATORIOS DROP TOKEN';

    IF (EXISTS (SELECT 1 FROM RDB$RELATION_FIELDS
                WHERE RDB$RELATION_NAME = 'PRACTICAS' AND RDB$FIELD_NAME = 'ENABLE_PASSWORD')) THEN
        EXECUTE STATEMENT 'ALTER TABLE PRACTICAS DROP ENABLE_PASSWORD';

    -- Latido de servicios externos a la API (el agente raccoon-accesos)
    IF (NOT EXISTS (SELECT 1 FROM RDB$RELATIONS WHERE RDB$RELATION_NAME = 'LATIDOS')) THEN
        EXECUTE STATEMENT '
            CREATE TABLE LATIDOS (
                SERVICIO  VARCHAR(30)  NOT NULL PRIMARY KEY,
                VISTO     TIMESTAMP    NOT NULL
            )';
END^

SET TERM ; ^
COMMIT;
