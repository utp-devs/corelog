import { expect, type APIRequestContext } from '@playwright/test';

export interface ApiBoardCard {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
}

export interface ApiBoardColumn {
  readonly id: string;
  readonly name: string;
  readonly min_issues: number | null;
  readonly max_issues: number | null;
  readonly status_ids: string[];
  readonly cards: ApiBoardCard[];
}

export interface ApiBoardView {
  readonly board: { id: string; swimlane_mode: string };
  readonly columns: ApiBoardColumn[];
}

/** Id of the main board of a project. */
export async function mainBoardId(client: APIRequestContext, projectId: string): Promise<string> {
  const response = await client.get(`projects/${projectId}/boards`);
  expect(response.ok(), 'listing the boards').toBe(true);
  const boards = ((await response.json()) as { data: Array<{ id: string; is_default: boolean }> })
    .data;
  const main = boards.find((board) => board.is_default) ?? boards[0];
  expect(main, 'the project has a board').toBeTruthy();
  return main.id;
}

/** The view of a board as the API serves it (cards per column). */
export async function boardViewViaApi(
  client: APIRequestContext,
  boardId: string,
  query = '',
): Promise<ApiBoardView> {
  const response = await client.get(`boards/${boardId}/view${query}`);
  expect(response.ok(), 'reading the board view').toBe(true);
  return ((await response.json()) as { data: ApiBoardView }).data;
}

/** Keys of the cards in each column, in column order. */
export async function cardKeysByColumn(
  client: APIRequestContext,
  boardId: string,
): Promise<string[][]> {
  const view = await boardViewViaApi(client, boardId);
  return view.columns.map((column) => column.cards.map((card) => card.key));
}

/** Sets the WIP limit of one column (by position) and keeps the rest of the board as it is. */
export async function setColumnLimitViaApi(
  client: APIRequestContext,
  boardId: string,
  position: number,
  max: number | null,
): Promise<void> {
  const view = await boardViewViaApi(client, boardId);
  const columns = view.columns.map((column, index) => ({
    id: column.id,
    name: column.name,
    min_issues: column.min_issues,
    max_issues: index === position ? max : column.max_issues,
    status_ids: column.status_ids,
  }));
  const response = await client.put(`boards/${boardId}/columns`, { data: { columns } });
  expect(response.ok(), `saving the columns (${await response.text()})`).toBe(true);
}

export async function setQuickFiltersViaApi(
  client: APIRequestContext,
  boardId: string,
  filters: Array<{ name: string; query: string }>,
): Promise<void> {
  const response = await client.put(`boards/${boardId}/quick-filters`, {
    data: { quick_filters: filters },
  });
  expect(response.ok(), `saving the quick filters (${await response.text()})`).toBe(true);
}

export interface ApiSprint {
  readonly id: string;
  readonly name: string;
  readonly status: 'planned' | 'active' | 'closed';
}

export async function listSprintsViaApi(
  client: APIRequestContext,
  projectId: string,
): Promise<ApiSprint[]> {
  const response = await client.get('sprints', { params: { project_id: projectId } });
  expect(response.ok(), 'listing the sprints').toBe(true);
  return ((await response.json()) as { data: ApiSprint[] }).data;
}

/** Deletes the planned sprints a test created (a project delete also removes them; this is for early cleanup). */
export async function deletePlannedSprintsViaApi(
  client: APIRequestContext,
  projectId: string,
): Promise<void> {
  for (const sprint of await listSprintsViaApi(client, projectId)) {
    if (sprint.status === 'planned') await client.delete(`sprints/${sprint.id}`);
  }
}
