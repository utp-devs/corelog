import { expect, type APIRequestContext } from '@playwright/test';

export interface ApiWorkflowStatus {
  readonly id: string;
  readonly name: string;
  readonly category: 'todo' | 'in_progress' | 'done';
  readonly issue_count: number;
}

export interface ApiWorkflowTransition {
  readonly id: string;
  readonly name: string;
  readonly from: string[];
  readonly to: string;
  readonly conditions: unknown[];
  readonly validators: unknown[];
  readonly post_functions: unknown[];
  readonly required_fields: string[];
}

export interface ApiWorkflow {
  readonly statuses: ApiWorkflowStatus[];
  readonly initial_status_id: string;
  readonly transitions: ApiWorkflowTransition[];
  readonly version: number;
}

export async function readWorkflow(
  client: APIRequestContext,
  projectId: string,
): Promise<ApiWorkflow> {
  const response = await client.get(`projects/${projectId}/workflow`);
  expect(response.ok(), 'reading the workflow').toBe(true);
  return ((await response.json()) as { data: ApiWorkflow }).data;
}

/** Adds a transition (with any rules) to the workflow of a project through the API. */
export async function addTransitionViaApi(
  client: APIRequestContext,
  projectId: string,
  transition: Partial<ApiWorkflowTransition> & { id: string; name: string; to: string },
): Promise<ApiWorkflow> {
  const current = await readWorkflow(client, projectId);
  const response = await client.put(`projects/${projectId}/workflow`, {
    data: {
      expected_version: current.version,
      initial_ref: current.initial_status_id,
      statuses: current.statuses.map((status) => ({ ref: status.id, id: status.id })),
      transitions: [
        ...current.transitions,
        {
          from: [],
          conditions: [],
          validators: [],
          post_functions: [],
          required_fields: [],
          ...transition,
        },
      ],
      status_mapping: {},
    },
  });
  expect(response.ok(), `saving the workflow (${response.status()})`).toBe(true);
  return ((await response.json()) as { data: ApiWorkflow }).data;
}

export interface ApiIssue {
  readonly id: string;
  readonly key: string;
}

export async function createIssueViaApi(
  client: APIRequestContext,
  projectId: string,
  summary: string,
  extra: Record<string, unknown> = {},
): Promise<ApiIssue> {
  const response = await client.post('issues', {
    data: { project_id: projectId, summary, description: 'Creada por una prueba e2e.', ...extra },
  });
  expect(response.status(), `creating the issue (${await response.text()})`).toBe(201);
  const body = (await response.json()) as { data: ApiIssue };
  return { id: body.data.id, key: body.data.key };
}

export async function userIdOf(client: APIRequestContext): Promise<string> {
  const response = await client.get('users/me');
  return ((await response.json()) as { data: { id: string } }).data.id;
}
