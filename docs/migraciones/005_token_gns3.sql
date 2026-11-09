-- =====================================================================
--  Migración 005: el GNS3 de cada lab vuelve a pedir usuario y contraseña
--  (usuario = matrícula del dueño, contraseña = token de 8 caracteres).
--  Idempotente (raccoon.sh la corre siempre) y segura con datos:
--   - LABORATORIOS.TOKEN se agrega si no existe (puede quedar vacío).
--   - Los labs creados mientras GNS3 no pedía contraseña no tienen token:
--     se les deja un aviso para que el dueño los limpie y los cree de nuevo
--     (sus proyectos no se pierden: están en gns3_projects).
-- =====================================================================
SET TERM ^ ;

EXECUTE BLOCK AS
BEGIN
    IF (NOT EXISTS (SELECT 1 FROM RDB$RELATION_FIELDS
                    WHERE RDB$RELATION_NAME = 'LABORATORIOS' AND RDB$FIELD_NAME = 'TOKEN')) THEN
        EXECUTE STATEMENT 'ALTER TABLE LABORATORIOS ADD TOKEN VARCHAR(16)';
END^

SET TERM ; ^
COMMIT;

UPDATE LABORATORIOS SET MENSAJE = 'Este laboratorio se creó sin contraseña de GNS3. Límpialo y créalo de nuevo; tus proyectos se conservan.'
WHERE TOKEN IS NULL OR TOKEN = '';
COMMIT;
