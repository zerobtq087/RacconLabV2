-- =====================================================================
--  Migración 004: prácticas sin "contraseña de enable" y latido del agente.
--  Idempotente (raccoon.sh la corre siempre) y segura con datos:
--   - PRACTICAS.ENABLE_PASSWORD: las contraseñas ahora van en las
--     instrucciones y en los criterios ("enable secret ...").
--   - LATIDOS: cuándo avisó por última vez el agente del firewall.
--  (La columna LABORATORIOS.TOKEN ya NO se toca aquí: el GNS3 de los labs
--   volvió a pedir contraseña, ver 005_token_gns3.sql.)
-- =====================================================================
SET TERM ^ ;

EXECUTE BLOCK AS
BEGIN
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
