echo "" > import http from './http'

/*
* Laboratorios GNS3: TODO es real (Docker + Firebird). Ya no usa mock.
*/
export const labsApi = {
async estado() {
return (await http.get('/labs/estado')).data
},

/** { es_colaborativo: bool } o { codigo_lab: 'COLAB-XXXX' } */
async crear(payload) {
// Levantar GNS3 y cargar plantillas puede tardar ~2 min
return (await http.post('/labs/crear', payload, { timeout: 300000 })).data
},

async limpiar() {
return (await http.post('/labs/limpiar', {}, { timeout: 120000 })).data
},

// ----- Equipo -----
/** Compañeros de mi grupo: [{ matricula, nombre, ocupado }] */
async companeros() {
return (await http.get('/labs/companeros')).data
},

async invitar(matricula) {
return (await http.post('/labs/invitaciones/invitar', { invitados_ids: [matricula] })).data
},

/** Invitaciones que recibí: [{ id_invitacion, emisor, nombre_emisor, id_workspace }] */
async pendientes() {
return (await http.get('/labs/invitaciones/pendientes')).data
},

async responder(idInvitacion, aceptar) {
return (await http.post('/labs/invitaciones/responder', { id_invitacion: idInvitacion, aceptar })).data
}
}
