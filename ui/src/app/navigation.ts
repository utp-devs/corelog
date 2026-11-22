export type NavigationRole = 'admin' | 'agent' | 'requester'

export type NavigationItem = {
  key: string
  label: string
  icon: 'dashboard' | 'tickets' | 'profile' | 'teams' | 'calendar' | 'projects'
  roles: NavigationRole[]
}

export const navigationItems: NavigationItem[] = [
  {
    key: '/projects',
    label: 'Proyectos',
    icon: 'projects',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/issues',
    label: 'Incidencias',
    icon: 'tickets',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/profile',
    label: 'Mi perfil',
    icon: 'profile',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/teams',
    label: 'Equipos',
    icon: 'teams',
    roles: ['admin'],
  },
  {
    key: '/dashboard',
    label: 'Inicio',
    icon: 'dashboard',
    roles: ['admin', 'agent', 'requester'],
  },
  {
    key: '/calendar',
    label: 'Calendario',
    icon: 'calendar',
    roles: ['admin', 'agent', 'requester'],
  },
]
