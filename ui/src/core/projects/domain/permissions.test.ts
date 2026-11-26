import { describe, expect, it } from 'vitest'

import { effectiveProjectRole, hasProjectPermission, projectPermissions } from './permissions'

describe('matriz de permisos de proyecto', () => {
  it('concede a cada rol de miembro los permisos que define el backend', () => {
    const expected: Record<string, string[]> = {
      lead: ['view_project', 'watch_issue', 'comment', 'create_issue', 'edit_issue', 'move_cards', 'manage_sprint', 'manage_board', 'manage_workflow', 'edit_project', 'manage_members'],
      contributor: ['view_project', 'watch_issue', 'comment', 'create_issue', 'edit_issue', 'move_cards'],
      viewer: ['view_project', 'watch_issue'],
    }
    for (const [role, permissions] of Object.entries(expected)) {
      const granted = projectPermissions.filter((item) => hasProjectPermission(role as 'lead', item.key)).map((item) => item.key)
      expect(granted.sort()).toEqual([...permissions].sort())
    }
  })

  it('solo el administrador global puede eliminar el proyecto', () => {
    expect(hasProjectPermission('admin', 'delete_project')).toBe(true)
    for (const role of ['lead', 'contributor', 'viewer', null] as const) {
      expect(hasProjectPermission(role, 'delete_project')).toBe(false)
    }
  })

  it('sin membresia no concede nada, salvo al administrador global', () => {
    expect(hasProjectPermission(null, 'view_project')).toBe(false)
    expect(hasProjectPermission('admin', 'manage_members')).toBe(true)
  })

  it('calcula el rol efectivo desde el rol global y la membresia', () => {
    const members = [{ userId: 'u1', role: 'contributor' as const }]
    expect(effectiveProjectRole('user', 'u1', members)).toBe('contributor')
    expect(effectiveProjectRole('user', 'u2', members)).toBeNull()
    expect(effectiveProjectRole('admin', 'u2', members)).toBe('admin')
    expect(effectiveProjectRole('user', undefined, members)).toBeNull()
  })
})
