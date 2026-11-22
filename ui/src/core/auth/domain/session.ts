// Sesion de login de la cuenta propia. El backend marca cual es la de la peticion actual.
export interface AccountSession {
  id: string
  userAgent: string
  createdAt: string
  lastSeenAt: string
  current: boolean
}

// Resume la cadena de agente de usuario en "Navegador · Sistema" para mostrarla al usuario.
// Las cadenas que no reconoce se muestran tal cual, recortadas, para no ocultar informacion util (por ejemplo, curl).
export function describeUserAgent(userAgent: string): string {
  const ua = userAgent.trim()
  if (!ua) return 'Navegador sin identificar'

  const browser = /Edg\//.test(ua) ? 'Edge'
    : /OPR\/|Opera/.test(ua) ? 'Opera'
    : /Firefox\//.test(ua) ? 'Firefox'
    : /Chrome\/|HeadlessChrome\//.test(ua) ? 'Chrome'
    : /Safari\//.test(ua) ? 'Safari'
    : null
  const system = /Windows/.test(ua) ? 'Windows'
    : /Android/.test(ua) ? 'Android'
    : /iPhone|iPad/.test(ua) ? 'iOS'
    : /Mac OS X|Macintosh/.test(ua) ? 'macOS'
    : /Linux/.test(ua) ? 'Linux'
    : null

  if (browser && system) return `${browser} · ${system}`
  if (browser) return browser
  return ua.length > 60 ? `${ua.slice(0, 57)}...` : ua
}
