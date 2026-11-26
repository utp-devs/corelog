import { afterEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import { setOrgIdProvider } from '@/shared/lib/org-context'
import { apiClient } from './api-client'
import { TeamHttpRepository } from './team-http-repository'

const ORG_ID = '00000000-0000-4000-8000-0000000000aa'

setOrgIdProvider(() => ORG_ID)

const teamDto = {
  id: 'team-1',
  organization_id: ORG_ID,
  name: 'Plataforma',
  description: 'Infra y datos',
  avatar_color: '#1F6FEB',
  created_by: 'user-1',
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-02T10:00:00Z',
}

const memberDto = {
  team_id: 'team-1',
  user_id: 'user-2',
  role: 'maintainer' as const,
  created_at: '2026-11-03T10:00:00Z',
}

describe('TeamHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists teams of the current organization and maps the new fields', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [teamDto] } })

    const [team] = await new TeamHttpRepository().list()

    expect(get).toHaveBeenCalledWith('/teams', { params: { organization_id: ORG_ID } })
    expect(team).toEqual({
      id: 'team-1',
      organizationId: ORG_ID,
      name: 'Plataforma',
      description: 'Infra y datos',
      avatarColor: '#1F6FEB',
      createdBy: 'user-1',
      createdAt: '2026-11-01T10:00:00Z',
      updatedAt: '2026-11-02T10:00:00Z',
    })
  })

  it('maps member_count and my_role when the API sends them', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [{ ...teamDto, member_count: 4, my_role: 'maintainer' }] } })

    const [team] = await new TeamHttpRepository().list()

    expect(team.memberCount).toBe(4)
    expect(team.myRole).toBe('maintainer')
  })

  it('leaves member_count and my_role unset when the API omits them', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [teamDto] } })

    const [team] = await new TeamHttpRepository().list()

    expect('memberCount' in team).toBe(false)
    expect('myRole' in team).toBe(false)
  })

  it('lists the teams where a person is a member, with explicit ids', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [teamDto] } })

    const teams = await new TeamHttpRepository().listForMember(ORG_ID, 'user-9')

    expect(get).toHaveBeenCalledWith('/teams', { params: { organization_id: ORG_ID, member_id: 'user-9' } })
    expect(teams[0].id).toBe('team-1')
  })

  it('sends organization_id in the body when creating a team', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { data: teamDto } })

    await new TeamHttpRepository().create({ organizationId: ORG_ID, name: 'Plataforma' })

    expect(post).toHaveBeenCalledWith('/teams', { organization_id: ORG_ID, name: 'Plataforma' })
  })

  it('sends only the provided fields on partial update', async () => {
    const patch = vi.spyOn(apiClient, 'patch').mockResolvedValue({ data: { data: teamDto } })

    await new TeamHttpRepository().update('team-1', { name: 'Plataforma core', avatarColor: null })

    expect(patch).toHaveBeenCalledWith('/teams/team-1', { name: 'Plataforma core', avatar_color: null })
  })

  it('lists members and maps roles', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [memberDto] } })

    const [member] = await new TeamHttpRepository().listMembers('team-1')

    expect(member).toEqual({ teamId: 'team-1', userId: 'user-2', role: 'maintainer', createdAt: '2026-11-03T10:00:00Z' })
  })

  it('reads the organization people with names for the members view', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [{ user_id: 'user-2', name: 'Luis Soto', email: 'luis@example.com', role: 'member', joined_at: '' }] },
    })

    const people = await new TeamHttpRepository().listOrganizationPeople(ORG_ID)

    expect(get).toHaveBeenCalledWith(`/organizations/${ORG_ID}/members`)
    expect(people).toEqual([{ userId: 'user-2', name: 'Luis Soto', email: 'luis@example.com' }])
  })

  it('omits role when adding a member so the server default applies', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { data: { ...memberDto, role: 'member' } } })

    await new TeamHttpRepository().addMember('team-1', 'user-2')

    expect(post).toHaveBeenCalledWith('/teams/team-1/members', { user_id: 'user-2' })
  })

  it('changes a member role with the maintainer or member value', async () => {
    const patch = vi.spyOn(apiClient, 'patch').mockResolvedValue({ data: { data: memberDto } })

    await new TeamHttpRepository().updateMemberRole('team-1', 'user-2', 'maintainer')

    expect(patch).toHaveBeenCalledWith('/teams/team-1/members/user-2', { role: 'maintainer' })
  })

  it('lets API error codes pass through unchanged', async () => {
    const failure = new AppError({ status: 409, code: 'team.member_exists', message: 'ya es miembro' })
    vi.spyOn(apiClient, 'post').mockRejectedValue(failure)

    const error = await new TeamHttpRepository().addMember('team-1', 'user-2').catch((e: unknown) => e)

    expect(error).toBe(failure)
    expect((error as AppError).code).toBe('team.member_exists')
  })
})
