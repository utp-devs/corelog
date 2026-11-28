import { expect, type APIRequestContext, type Page } from '@playwright/test';

import type { ApiForRole, DataHelpers } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createIssueViaApi, userIdOf, type ApiIssue } from '../../lib/workflow-api.ts';

export interface IssueProject {
  readonly id: string;
  readonly key: string;
  readonly name: string;
}

/**
 * A fresh project of the Acme organization with the lead, the member (contributor) and the viewer
 * added to it, plus one issue created by the owner. Everything is deleted when the test ends.
 */
export async function projectWithIssue(
  api: ApiForRole,
  data: DataHelpers,
  prefix = 'iss',
): Promise<{ owner: APIRequestContext; project: IssueProject; issue: ApiIssue }> {
  const owner = await api('owner');
  const acme = await orgIdBySlug(owner, 'acme');
  const project = await data.createProject('owner', acme, prefix);
  for (const [role, projectRole] of [
    ['lead', 'lead'],
    ['member', 'contributor'],
    ['viewer', 'viewer'],
  ] as const) {
    const added = await owner.post(`projects/${project.id}/members`, {
      data: { user_id: await userIdOf(await api(role)), role: projectRole },
    });
    expect(added.ok(), `adding ${role} to the project`).toBeTruthy();
  }
  const issue = await createIssueViaApi(owner, project.id, 'Incidencia de la vista');
  return { owner, project, issue };
}

/** Reads the full view of an issue through the API. */
export async function readView(
  client: APIRequestContext,
  ref: string,
): Promise<Record<string, unknown>> {
  const response = await client.get(`issues/${ref}`);
  expect(response.ok(), 'reading the issue').toBe(true);
  return ((await response.json()) as { data: Record<string, unknown> }).data;
}

/** Opens the issue view and, on phones where the details card starts collapsed, expands it. */
export async function openIssueView(page: Page, key: string): Promise<void> {
  await page.goto(appPath(`/browse/${key}`));
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  const toggle = page.getByRole('button', { name: 'Mostrar detalles' });
  if (await toggle.isVisible()) {
    await toggle.click();
  }
  await expect(page.getByTestId('issue-fields')).toBeVisible();
}
