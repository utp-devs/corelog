export type NavigationRole = 'admin' | 'agent' | 'requester'

export type NavigationGroup = 'Trabajo' | 'Administración' | 'Cuenta'

export type NavigationItem = {
  key: string
  label: string
  icon: 'dashboard' | 'tickets' | 'profile' | 'teams' | 'projects'
  group: NavigationGroup
  roles: NavigationRole[]
}

// Fuente unica del menu lateral. El calendario no esta aqui: es por proyecto y se abre desde el proyecto.
export const navigationItems: NavigationItem[] = [
  {
    key: '/dashboard',
    label: 'Mi resumen',
    icon: 'dashboard',
    group: 'Trabajo',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/projects',
    label: 'Proyectos',
    icon: 'projects',
    group: 'Trabajo',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/issues',
    label: 'Incidencias',
    icon: 'tickets',
    group: 'Trabajo',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/teams',
    label: 'Equipos',
    icon: 'teams',
    group: 'Administración',
    roles: ['admin'],
  },
  {
    key: '/profile',
    label: 'Mi perfil',
    icon: 'profile',
    group: 'Cuenta',
    roles: ['admin', 'agent', 'requester'],
  },
]
