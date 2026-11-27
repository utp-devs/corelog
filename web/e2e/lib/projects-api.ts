import type { APIRequestContext } from '@playwright/test';

export interface ApiProject {
  readonly id: string;
  readonly key: string;
  readonly name: string;
}

/** Random project key: an upper-case letter followed by four upper-case letters or digits. */
export function randomProjectKey(): string {
  let tail = '';
  while (tail.length < 4) tail += Math.random().toString(36).slice(2).toUpperCase();
  return `W${tail.slice(0, 4).replace(/[^A-Z0-9]/g, 'X')}`;
}

/** Creates an open project through the API as the caller. */
export async function createProjectViaApi(
  client: APIRequestContext,
  organizationId: string,
  name: string,
  key: string = randomProjectKey(),
): Promise<ApiProject> {
  const response = await client.post('projects', {
    data: {
      organization_id: organizationId,
      key,
      name,
      description: '',
      type: 'kanban',
      template: 'kanban',
      visibility: 'open',
    },
  });
  if (response.status() !== 201) {
    throw new Error(`POST projects returned ${response.status()}: ${await response.text()}`);
  }
  const body = (await response.json()) as { data: ApiProject };
  return { id: body.data.id, key: body.data.key, name: body.data.name };
}

/** Finds a project by key in the organization, or null when it does not exist. */
export async function findProjectByKey(
  client: APIRequestContext,
  organizationId: string,
  key: string,
): Promise<ApiProject | null> {
  const response = await client.get(`projects/${key}`, {
    params: { organization_id: organizationId },
  });
  if (response.status() !== 200) return null;
  const body = (await response.json()) as { data: ApiProject };
  return { id: body.data.id, key: body.data.key, name: body.data.name };
}

/** Deletes a project (the server asks for its exact name). A project already gone is fine. */
export async function deleteProjectViaApi(
  client: APIRequestContext,
  project: ApiProject,
): Promise<void> {
  const response = await client.delete(`projects/${project.id}`, {
    data: { confirmation: project.name },
  });
  if (![200, 204, 404].includes(response.status())) {
    throw new Error(`DELETE projects/${project.id} returned ${response.status()}`);
  }
}
