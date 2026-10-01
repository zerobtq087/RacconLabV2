// Utilidades IPv4 para vista previa (el cálculo oficial lo hace el backend Go)

export const IP_REGEX = /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/

const aNumero = (ip) => ip.split('.').reduce((acc, o) => (acc << 8) + Number(o), 0) >>> 0
const aTexto = (n) => [24, 16, 8, 0].map((s) => (n >>> s) & 255).join('.')
const mascara = (prefijo) => (prefijo === 0 ? 0 : (0xffffffff << (32 - prefijo)) >>> 0)

export const esIPValida = (ip) => IP_REGEX.test(String(ip || '').trim())

/** Datos de una red: { red, mascara, broadcast, primera, ultima, hosts, esRed } */
export const calcularRed = (ip, prefijo) => {
  if (!esIPValida(ip) || prefijo < 0 || prefijo > 32) return null
  const n = aNumero(ip)
  const m = mascara(prefijo)
  const red = (n & m) >>> 0
  const broadcast = (red | (~m >>> 0)) >>> 0
  const hosts = prefijo >= 31 ? (prefijo === 32 ? 1 : 2) : broadcast - red - 1
  return {
    red: aTexto(red),
    mascara: aTexto(m),
    broadcast: aTexto(broadcast),
    primera: aTexto(prefijo >= 31 ? red : red + 1),
    ultima: aTexto(prefijo >= 31 ? broadcast : broadcast - 1),
    hosts,
    esRed: n === red // true si la IP escrita es la dirección de red
  }
}

/** Divide una red en N subredes de un prefijo dado */
export const subdividir = (ip, prefijo, nuevoPrefijo, cantidad) => {
  const base = calcularRed(ip, prefijo)
  if (!base || nuevoPrefijo < prefijo) return []
  const inicio = aNumero(base.red)
  const salto = 2 ** (32 - nuevoPrefijo)
  const maximo = 2 ** (nuevoPrefijo - prefijo)
  return Array.from({ length: Math.min(cantidad, maximo) }, (_, i) => ({
    red: `${aTexto((inicio + i * salto) >>> 0)}/${nuevoPrefijo}`,
    ...calcularRed(aTexto((inicio + i * salto) >>> 0), nuevoPrefijo)
  }))
}