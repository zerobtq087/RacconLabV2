<template>
  <AppFrame>
    <v-card border class="my-5" elevation="0" rounded="lg" title="Calculadora de redes">
      <v-tabs v-model="pestana" color="primary" show-arrows>
        <v-tab prepend-icon="mdi-ip-network-outline" text="Subred" value="subred" />
        <v-tab prepend-icon="mdi-call-split" text="Dividir" value="dividir" />
        <v-tab prepend-icon="mdi-table-large" text="VLSM" value="vlsm" />
        <v-tab prepend-icon="mdi-swap-horizontal" text="Enlaces /30 /31" value="enlaces" />
        <v-tab prepend-icon="mdi-call-merge" text="Sumarización" value="sumarizar" />
        <v-tab prepend-icon="mdi-swap-vertical" text="Conversión" value="convertir" />
      </v-tabs>

      <v-card-text>
        <!-- Error de la pestaña actual (dentro de la página, sin ventanas) -->
        <v-alert v-if="error" class="mb-4" closable :text="error" type="error" variant="tonal" @click:close="error = ''" />

        <v-tabs-window v-model="pestana">
          <!-- 1) Subred -->
          <v-tabs-window-item value="subred">
            <v-row dense>
              <v-col cols="12" md="5">
                <v-text-field v-model="fSubred.ip" hint="También puedes escribir 192.168.1.130/26" label="Dirección IP" variant="outlined" @keyup.enter="calcularSubred" />
              </v-col>
              <v-col cols="8" md="4">
                <v-text-field v-model="fSubred.mascara" hint="26, /26 o 255.255.255.192" label="Prefijo o máscara" variant="outlined" @keyup.enter="calcularSubred" />
              </v-col>
              <v-col cols="4" md="3">
                <v-btn block color="primary" :loading="cargando" size="large" text="Calcular" variant="flat" @click="calcularSubred" />
              </v-col>
            </v-row>

            <template v-if="rSubred">
              <v-table class="mt-2" density="compact">
                <tbody>
                  <tr v-for="f in filasSubred" :key="f.titulo">
                    <td class="text-medium-emphasis" style="width: 40%">{{ f.titulo }}</td>
                    <td class="font-weight-medium">{{ f.valor }}</td>
                    <td class="text-right" style="width: 48px"><BotonCopiar :texto="String(f.valor)" /></td>
                  </tr>
                </tbody>
              </v-table>

              <div class="d-flex align-center mt-4 mb-1">
                <span class="text-subtitle-2">Vista en binario</span>
                <span class="text-caption text-medium-emphasis ms-2">(en color, los {{ rSubred.prefijo }} bits de red)</span>
                <v-spacer />
                <BotonCopiar etiqueta="Copiar todo" :texto="textoSubred" />
              </div>
              <v-table density="compact">
                <tbody>
                  <tr v-for="b in binarios" :key="b.titulo">
                    <td class="text-medium-emphasis" style="width: 20%">{{ b.titulo }}</td>
                    <td class="text-no-wrap" style="font-family: monospace">
                      <span v-for="(bit, i) in pintarBits(b.valor, rSubred.prefijo)" :key="i" :class="bit.red ? 'text-primary font-weight-bold' : ''">{{ bit.c }}</span>
                    </td>
                    <td class="text-right" style="width: 48px"><BotonCopiar :texto="b.valor" /></td>
                  </tr>
                </tbody>
              </v-table>
            </template>
          </v-tabs-window-item>

          <!-- 2) Dividir -->
          <v-tabs-window-item value="dividir">
            <v-row dense>
              <v-col cols="12" md="4">
                <v-text-field v-model="fDividir.red" label="Red" placeholder="192.168.0.0/24" variant="outlined" />
              </v-col>
              <v-col cols="6" md="3">
                <v-select v-model="fDividir.modo" item-title="text" item-value="value" :items="modosDividir" label="Dividir por" variant="outlined" />
              </v-col>
              <v-col cols="6" md="2">
                <v-text-field v-model.number="fDividir.cantidad" :label="fDividir.modo == 'subredes' ? 'Subredes' : 'Hosts por subred'" min="1" type="number" variant="outlined" />
              </v-col>
              <v-col cols="12" md="3">
                <v-btn block color="primary" :loading="cargando" size="large" text="Calcular" variant="flat" @click="calcularDividir" />
              </v-col>
            </v-row>

            <template v-if="rDividir">
              <div class="d-flex flex-wrap align-center ga-2 mb-2">
                <v-chip label :text="`Nuevo prefijo: /${rDividir.nuevo_prefijo} (${rDividir.mascara})`" />
                <v-chip label :text="`${rDividir.total} subredes`" />
                <v-chip label :text="`${rDividir.hosts_por_red} hosts por subred`" />
                <v-spacer />
                <BotonCopiar etiqueta="Copiar tabla" :texto="tablaTexto(columnasSubred, rDividir.subredes)" />
              </div>
              <v-alert v-if="rDividir.nota" class="mb-2" density="compact" :text="rDividir.nota" type="info" variant="tonal" />
              <v-data-table density="compact" :headers="encabezados(columnasSubred)" :items="rDividir.subredes" items-per-page="16" />
            </template>
          </v-tabs-window-item>

          <!-- 3) VLSM -->
          <v-tabs-window-item value="vlsm">
            <v-row dense>
              <v-col cols="12" md="4">
                <v-text-field v-model="fVLSM.red" label="Red" placeholder="192.168.1.0/24" variant="outlined" />
              </v-col>
            </v-row>
            <div class="text-subtitle-2 mb-2">Subredes que necesitas</div>
            <v-row v-for="(n, i) in fVLSM.necesidades" :key="i" dense>
              <v-col cols="6" md="4"><v-text-field v-model="n.nombre" density="compact" label="Nombre" variant="outlined" /></v-col>
              <v-col cols="4" md="2"><v-text-field v-model.number="n.hosts" density="compact" label="Hosts" min="1" type="number" variant="outlined" /></v-col>
              <v-col cols="2" md="1">
                <v-btn color="error" :disabled="fVLSM.necesidades.length == 1" icon="mdi-delete" size="small" variant="text" @click="fVLSM.necesidades.splice(i, 1)" />
              </v-col>
            </v-row>
            <div class="d-flex flex-wrap ga-2 mb-4">
              <v-btn prepend-icon="mdi-plus" text="Agregar subred" variant="tonal" @click="fVLSM.necesidades.push({ nombre: '', hosts: 10 })" />
              <v-btn color="primary" :loading="cargando" text="Calcular VLSM" variant="flat" @click="calcularVLSM" />
            </div>

            <template v-if="rVLSM">
              <div class="d-flex flex-wrap align-center ga-2 mb-2">
                <v-chip label :text="`Usadas: ${rVLSM.usadas} de ${rVLSM.total}`" />
                <v-chip color="success" label :text="`Sobran: ${rVLSM.libres} (${rVLSM.porcentaje_libre.toFixed(1)}%)`" />
                <v-spacer />
                <BotonCopiar etiqueta="Copiar tabla" :texto="tablaTexto(columnasVLSM, rVLSM.subredes)" />
              </div>
              <v-data-table density="compact" :headers="encabezados(columnasVLSM)" :items="rVLSM.subredes" items-per-page="-1" />
              <div v-if="rVLSM.bloques_libres.length" class="mt-3">
                <span class="text-subtitle-2">Espacio libre:</span>
                <v-chip v-for="b in rVLSM.bloques_libres" :key="b" class="ms-2" label size="small" :text="b" />
                <BotonCopiar :texto="rVLSM.bloques_libres.join('\n')" />
              </div>
            </template>
          </v-tabs-window-item>

          <!-- 4) Enlaces punto a punto -->
          <v-tabs-window-item value="enlaces">
            <v-row dense>
              <v-col cols="12" md="4">
                <v-text-field v-model="fEnlaces.red" label="Red para los enlaces" placeholder="10.0.0.0/24" variant="outlined" />
              </v-col>
              <v-col cols="6" md="2">
                <v-select v-model="fEnlaces.prefijo" :items="[30, 31]" label="Prefijo" prefix="/" variant="outlined" />
              </v-col>
              <v-col cols="6" md="3">
                <v-text-field v-model.number="fEnlaces.cantidad" hint="0 = todos los que quepan" label="Enlaces" min="0" type="number" variant="outlined" />
              </v-col>
              <v-col cols="12" md="3">
                <v-btn block color="primary" :loading="cargando" size="large" text="Calcular" variant="flat" @click="calcularEnlaces" />
              </v-col>
            </v-row>

            <template v-if="rEnlaces">
              <div class="d-flex flex-wrap align-center ga-2 mb-2">
                <v-chip label :text="`Máscara: ${rEnlaces.mascara}`" />
                <v-chip label :text="`Caben ${rEnlaces.caben} enlaces /${rEnlaces.prefijo}`" />
                <v-spacer />
                <BotonCopiar etiqueta="Copiar tabla" :texto="tablaTexto(columnasEnlaces, rEnlaces.enlaces)" />
              </div>
              <v-alert v-if="rEnlaces.nota" class="mb-2" density="compact" :text="rEnlaces.nota" type="info" variant="tonal" />
              <v-data-table density="compact" :headers="encabezados(columnasEnlaces)" :items="rEnlaces.enlaces" items-per-page="16" />
            </template>
          </v-tabs-window-item>

          <!-- 5) Sumarización -->
          <v-tabs-window-item value="sumarizar">
            <v-row dense>
              <v-col cols="12" md="6">
                <v-textarea v-model="fSumarizar.redes" hint="Una red por renglón (o separadas por comas)" label="Redes" rows="5" variant="outlined" />
              </v-col>
              <v-col cols="12" md="3">
                <v-btn block color="primary" :loading="cargando" size="large" text="Sumarizar" variant="flat" @click="calcularSumarizar" />
              </v-col>
            </v-row>

            <template v-if="rSumarizar">
              <v-table density="compact">
                <tbody>
                  <tr v-for="f in filasSumarizar" :key="f.titulo">
                    <td class="text-medium-emphasis" style="width: 40%">{{ f.titulo }}</td>
                    <td class="font-weight-medium">{{ f.valor }}</td>
                    <td class="text-right" style="width: 48px"><BotonCopiar :texto="String(f.valor)" /></td>
                  </tr>
                </tbody>
              </v-table>
              <v-alert
                class="mt-3"
                density="compact"
                :text="rSumarizar.exacta
                  ? 'Sumarización exacta: la superred cubre justo esas redes.'
                  : `La superred también cubre ${rSumarizar.direcciones_extra} direcciones que no están en tu lista (no es exacta).`"
                :type="rSumarizar.exacta ? 'success' : 'warning'"
                variant="tonal"
              />
            </template>
          </v-tabs-window-item>

          <!-- 6) Conversión -->
          <v-tabs-window-item value="convertir">
            <v-row dense>
              <v-col cols="6" md="3">
                <v-select v-model="fConvertir.tipo" item-title="text" item-value="value" :items="tiposConversion" label="Tengo un(a)" variant="outlined" />
              </v-col>
              <v-col cols="6" md="4">
                <v-text-field v-model="fConvertir.valor" :placeholder="ejemploConversion" label="Valor" variant="outlined" @keyup.enter="calcularConvertir" />
              </v-col>
              <v-col cols="12" md="3">
                <v-btn block color="primary" :loading="cargando" size="large" text="Convertir" variant="flat" @click="calcularConvertir" />
              </v-col>
            </v-row>

            <v-table v-if="rConvertir" density="compact">
              <tbody>
                <tr v-for="f in filasConvertir" :key="f.titulo">
                  <td class="text-medium-emphasis" style="width: 40%">{{ f.titulo }}</td>
                  <td class="font-weight-medium" :style="f.mono ? 'font-family: monospace' : ''">{{ f.valor }}</td>
                  <td class="text-right" style="width: 48px"><BotonCopiar :texto="String(f.valor)" /></td>
                </tr>
              </tbody>
            </v-table>
          </v-tabs-window-item>
        </v-tabs-window>
      </v-card-text>
    </v-card>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import BotonCopiar from '../components/BotonCopiar.vue'
import { computed, reactive, ref, watch } from 'vue'
import axios from 'axios'
import type {
  InfoSubred, Necesidad, ResultadoConversion, ResultadoDivision, ResultadoEnlaces,
  ResultadoSumarizacion, ResultadoVLSM,
} from '@/models/Calculadora'

const pestana = ref('subred')
const cargando = ref(false)
const error = ref('')
watch(pestana, () => error.value = '')

// Formularios (con ejemplos para empezar)
const fSubred = reactive({ ip: '192.168.1.130', mascara: '26' })
const fDividir = reactive({ red: '192.168.0.0/24', modo: 'subredes', cantidad: 4 })
const fVLSM = reactive<{ red: string, necesidades: Necesidad[] }>({
  red: '192.168.1.0/24',
  necesidades: [{ nombre: 'Ventas', hosts: 100 }, { nombre: 'Sistemas', hosts: 50 }, { nombre: 'Gerencia', hosts: 25 }, { nombre: 'Enlace', hosts: 2 }],
})
const fEnlaces = reactive({ red: '10.0.0.0/28', prefijo: 30, cantidad: 0 })
const fSumarizar = reactive({ redes: '172.16.0.0/24\n172.16.1.0/24\n172.16.2.0/24\n172.16.3.0/24' })
const fConvertir = reactive({ tipo: 'prefijo', valor: '/20' })

const modosDividir = [
  { text: 'Número de subredes', value: 'subredes' },
  { text: 'Hosts por subred', value: 'hosts' },
]
const tiposConversion = [
  { text: 'Prefijo', value: 'prefijo' },
  { text: 'Máscara', value: 'mascara' },
  { text: 'Wildcard', value: 'wildcard' },
]
const ejemploConversion = computed(() => ({ prefijo: '/20', mascara: '255.255.240.0', wildcard: '0.0.15.255' } as Record<string, string>)[fConvertir.tipo])

// Resultados
const rSubred = ref<InfoSubred | null>(null)
const rDividir = ref<ResultadoDivision | null>(null)
const rVLSM = ref<ResultadoVLSM | null>(null)
const rEnlaces = ref<ResultadoEnlaces | null>(null)
const rSumarizar = ref<ResultadoSumarizacion | null>(null)
const rConvertir = ref<ResultadoConversion | null>(null)

// Todas las pestañas llaman igual al backend: POST /api/calculadora/<operación>
function calcular<T> (operacion: string, datos: object, destino: { value: T | null }) {
  cargando.value = true
  error.value = ''
  axios.post(`/api/calculadora/${operacion}`, datos)
    .then(response => destino.value = response.data)
    .catch(err => {
      destino.value = null
      error.value = err.response?.data || 'No se pudo calcular.'
    })
    .finally(() => cargando.value = false)
}

const calcularSubred = () => calcular('subred', fSubred, rSubred)
const calcularDividir = () => calcular('dividir', fDividir, rDividir)
const calcularVLSM = () => calcular('vlsm', fVLSM, rVLSM)
const calcularEnlaces = () => calcular('enlaces', fEnlaces, rEnlaces)
const calcularConvertir = () => calcular('convertir', fConvertir, rConvertir)
const calcularSumarizar = () => calcular('sumarizar', {
  redes: fSumarizar.redes.split(/[\s,;]+/).filter(r => r != ''),
}, rSumarizar)

// ---------------- Presentación ----------------
const filasSubred = computed(() => {
  const r = rSubred.value
  if (!r) return []
  return [
    { titulo: 'Red', valor: r.red },
    { titulo: 'Máscara', valor: `${r.mascara} (/${r.prefijo})` },
    { titulo: 'Wildcard', valor: r.wildcard },
    { titulo: 'Primer host', valor: r.primer_host },
    { titulo: 'Último host', valor: r.ultimo_host },
    { titulo: 'Broadcast', valor: r.prefijo >= 31 ? '— (no tiene)' : r.broadcast },
    { titulo: 'Hosts utilizables', valor: r.hosts.toLocaleString('es-MX') },
    { titulo: 'Total de direcciones', valor: r.direcciones.toLocaleString('es-MX') },
    { titulo: 'Clase', valor: r.clase },
    { titulo: 'Tipo de dirección', valor: r.tipo },
  ]
})

const binarios = computed(() => {
  const b = rSubred.value?.binario
  if (!b) return []
  return [
    { titulo: 'IP', valor: b.ip },
    { titulo: 'Máscara', valor: b.mascara },
    { titulo: 'Red', valor: b.red },
    { titulo: 'Broadcast', valor: b.broadcast },
  ]
})

const textoSubred = computed(() => [...filasSubred.value, ...binarios.value.map(b => ({ titulo: `${b.titulo} (binario)`, valor: b.valor }))]
  .map(f => `${f.titulo}: ${f.valor}`).join('\n'))

// Marca los primeros "prefijo" bits (los puntos no cuentan)
function pintarBits (binario: string, prefijo: number) {
  let n = 0
  return [...binario].map(c => {
    if (c == '.') return { c, red: false }
    n++
    return { c, red: n <= prefijo }
  })
}

const filasSumarizar = computed(() => {
  const r = rSumarizar.value
  if (!r) return []
  return [
    { titulo: 'Superred', valor: r.resumen },
    { titulo: 'Máscara', valor: r.mascara },
    { titulo: 'Wildcard', valor: r.wildcard },
    { titulo: 'Redes sumarizadas', valor: r.redes.join(', ') },
  ]
})

const filasConvertir = computed(() => {
  const r = rConvertir.value
  if (!r) return []
  return [
    { titulo: 'Prefijo', valor: `/${r.prefijo}`, mono: false },
    { titulo: 'Máscara', valor: r.mascara, mono: false },
    { titulo: 'Wildcard', valor: r.wildcard, mono: false },
    { titulo: 'Máscara en binario', valor: r.mascara_binaria, mono: true },
    { titulo: 'Direcciones por red', valor: r.direcciones.toLocaleString('es-MX'), mono: false },
    { titulo: 'Hosts utilizables', valor: r.hosts.toLocaleString('es-MX'), mono: false },
  ]
})

// Tablas: columnas → encabezados de v-data-table y texto para copiar (Excel)
type Columna = [titulo: string, clave: string]
const columnasSubred: Columna[] = [['Red', 'red'], ['Máscara', 'mascara'], ['Primer host', 'primer_host'], ['Último host', 'ultimo_host'], ['Broadcast', 'broadcast'], ['Hosts', 'hosts']]
const columnasVLSM: Columna[] = [['Nombre', 'nombre'], ['Pedidos', 'hosts_pedidos'], ['Disponibles', 'hosts'], ['Red', 'red'], ['Máscara', 'mascara'], ['Primer host', 'primer_host'], ['Último host', 'ultimo_host'], ['Broadcast', 'broadcast'], ['Sobran', 'desperdicio']]
const columnasEnlaces: Columna[] = [['Enlace', 'red'], ['IP lado A', 'ip_a'], ['IP lado B', 'ip_b'], ['Broadcast', 'broadcast']]

function encabezados (columnas: Columna[]) {
  return columnas.map(([title, key]) => ({ title, key, sortable: false }))
}

// Separado por tabuladores: se pega directo en Excel o en un documento
function tablaTexto (columnas: Columna[], filas: object[]) {
  const lineas = [columnas.map(c => c[0]).join('\t')]
  for (const f of filas as Record<string, unknown>[]) {
    lineas.push(columnas.map(([, clave]) => f[clave] ?? '').join('\t'))
  }
  return lineas.join('\n')
}
</script>
