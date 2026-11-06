// Resultados de la calculadora de redes (igual que backend/domain/calculadora.go)

export interface Binario {
  ip: string
  mascara: string
  red: string
  broadcast: string
}

export interface InfoSubred {
  ip: string
  red: string
  broadcast: string
  primer_host: string
  ultimo_host: string
  hosts: number
  direcciones: number
  prefijo: number
  mascara: string
  wildcard: string
  clase: string
  tipo: string
  privada: boolean
  binario: Binario
}

export interface SubredCalc {
  nombre?: string
  hosts_pedidos?: number
  red: string
  mascara: string
  primer_host: string
  ultimo_host: string
  broadcast: string
  hosts: number
  desperdicio?: number
}

export interface ResultadoDivision {
  red: string
  nuevo_prefijo: number
  mascara: string
  total: number
  hosts_por_red: number
  subredes: SubredCalc[]
  nota: string
}

export interface Necesidad {
  nombre: string
  hosts: number
}

export interface ResultadoVLSM {
  red: string
  subredes: SubredCalc[]
  total: number
  usadas: number
  libres: number
  porcentaje_libre: number
  bloques_libres: string[]
}

export interface EnlaceP2P {
  red: string
  ip_a: string
  ip_b: string
  broadcast?: string
}

export interface ResultadoEnlaces {
  red: string
  prefijo: number
  mascara: string
  caben: number
  enlaces: EnlaceP2P[]
  nota: string
}

export interface ResultadoSumarizacion {
  redes: string[]
  resumen: string
  mascara: string
  wildcard: string
  exacta: boolean
  direcciones_extra: number
}

export interface ResultadoConversion {
  prefijo: number
  mascara: string
  wildcard: string
  mascara_binaria: string
  direcciones: number
  hosts: number
}
