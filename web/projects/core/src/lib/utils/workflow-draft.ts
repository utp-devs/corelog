import {
  WORKFLOW_CATEGORIES,
  WorkflowCategory,
  WorkflowDefinitionInput,
  WorkflowRule,
  WorkflowTemplate,
  WorkflowTransition,
  WorkflowView,
} from '../models/workflow.model';

/** Status of the workflow being edited. Saved statuses use their id as ref; new ones `new-<n>`. */
export interface DraftStatus {
  readonly ref: string;
  /** Id of the saved status; null for a status that does not exist yet. */
  readonly id: string | null;
  readonly name: string;
  readonly category: WorkflowCategory;
  /** Issues of the project in this status (0 for new ones). */
  readonly issueCount: number;
}

/** Editable copy of a workflow: statuses in order, the initial status and the transitions. */
export interface WorkflowDraft {
  readonly statuses: readonly DraftStatus[];
  readonly initialRef: string;
  readonly transitions: readonly WorkflowTransition[];
}

export function draftFromView(view: WorkflowView): WorkflowDraft {
  const ordered = [...view.statuses].sort((a, b) => a.position - b.position);
  return {
    statuses: ordered.map((status) => ({
      ref: status.id,
      id: status.id,
      name: status.name,
      category: status.category,
      issueCount: status.issueCount,
    })),
    initialRef: view.initialStatusId,
    transitions: view.transitions,
  };
}

/** Draft of a template: every status is new (the server reuses the ones of the same name). */
export function draftFromTemplate(template: WorkflowTemplate): WorkflowDraft {
  return {
    statuses: template.statuses.map((status) => ({
      ref: status.ref,
      id: null,
      name: status.name,
      category: status.category,
      issueCount: 0,
    })),
    initialRef: template.initialRef,
    transitions: template.transitions,
  };
}

export function draftToInput(
  draft: WorkflowDraft,
  expectedVersion: number,
  statusMapping: Readonly<Record<string, string>> = {},
): WorkflowDefinitionInput {
  return {
    expectedVersion,
    initialRef: draft.initialRef,
    statuses: draft.statuses.map((status) =>
      status.id
        ? { ref: status.ref, id: status.id }
        : { ref: status.ref, name: status.name, category: status.category },
    ),
    transitions: draft.transitions,
    statusMapping,
  };
}

export function draftsEqual(a: WorkflowDraft, b: WorkflowDraft): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/** Saved statuses of the baseline that the draft no longer has, with their issue counts. */
export function removedStatuses(baseline: WorkflowDraft, draft: WorkflowDraft): DraftStatus[] {
  const kept = new Set(draft.statuses.map((status) => status.ref));
  return baseline.statuses.filter((status) => status.id !== null && !kept.has(status.ref));
}

function newRef(draft: WorkflowDraft): string {
  const used = new Set(draft.statuses.map((status) => status.ref));
  let n = 1;
  while (used.has(`new-${n}`)) n += 1;
  return `new-${n}`;
}

export function addStatus(
  draft: WorkflowDraft,
  input: { name: string; category: WorkflowCategory; id?: string },
): WorkflowDraft {
  if (input.id && draft.statuses.some((status) => status.id === input.id)) return draft;
  const status: DraftStatus = input.id
    ? { ref: input.id, id: input.id, name: input.name, category: input.category, issueCount: 0 }
    : { ref: newRef(draft), id: null, name: input.name, category: input.category, issueCount: 0 };
  const statuses = [...draft.statuses, status];
  return { ...draft, statuses, initialRef: draft.initialRef || status.ref };
}

export function renameStatus(draft: WorkflowDraft, ref: string, name: string): WorkflowDraft {
  return {
    ...draft,
    statuses: draft.statuses.map((status) => (status.ref === ref ? { ...status, name } : status)),
  };
}

export function setStatusCategory(
  draft: WorkflowDraft,
  ref: string,
  category: WorkflowCategory,
): WorkflowDraft {
  return {
    ...draft,
    statuses: draft.statuses.map((status) =>
      status.ref === ref ? { ...status, category } : status,
    ),
  };
}

/** Removes a status and everything that points to it: transitions into it and sources from it. */
export function removeStatus(draft: WorkflowDraft, ref: string): WorkflowDraft {
  const statuses = draft.statuses.filter((status) => status.ref !== ref);
  const transitions = draft.transitions.flatMap((transition) => {
    if (transition.to === ref) return [];
    if (!transition.from.includes(ref)) return [transition];
    // A transition that only left the removed status would otherwise become "from any status".
    const from = transition.from.filter((item) => item !== ref);
    return from.length > 0 ? [{ ...transition, from }] : [];
  });
  const initialRef = draft.initialRef === ref ? (statuses[0]?.ref ?? '') : draft.initialRef;
  return { statuses, transitions, initialRef };
}

export function moveStatus(draft: WorkflowDraft, from: number, to: number): WorkflowDraft {
  if (from === to || to < 0 || to >= draft.statuses.length || from < 0) return draft;
  const statuses = [...draft.statuses];
  const [moved] = statuses.splice(from, 1);
  statuses.splice(to, 0, moved);
  return { ...draft, statuses };
}

export function setInitial(draft: WorkflowDraft, ref: string): WorkflowDraft {
  return draft.statuses.some((status) => status.ref === ref)
    ? { ...draft, initialRef: ref }
    : draft;
}

function slug(text: string): string {
  const base = text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
  return base || 'transicion';
}

export function uniqueTransitionId(draft: WorkflowDraft, name: string): string {
  const used = new Set(draft.transitions.map((transition) => transition.id));
  const base = slug(name).slice(0, 56);
  let candidate = base;
  let n = 2;
  while (used.has(candidate)) {
    candidate = `${base}_${n}`;
    n += 1;
  }
  return candidate;
}

export function addTransition(
  draft: WorkflowDraft,
  input: { name: string; from: readonly string[]; to: string },
): { draft: WorkflowDraft; id: string } {
  const id = uniqueTransitionId(draft, input.name);
  const transition: WorkflowTransition = {
    id,
    name: input.name,
    from: input.from,
    to: input.to,
    conditions: [],
    validators: [],
    postFunctions: [],
    requiredFields: [],
  };
  return { draft: { ...draft, transitions: [...draft.transitions, transition] }, id };
}

export function duplicateTransition(
  draft: WorkflowDraft,
  id: string,
): { draft: WorkflowDraft; id: string } | null {
  const source = draft.transitions.find((transition) => transition.id === id);
  if (!source) return null;
  const name = `${source.name} (copia)`;
  const copyId = uniqueTransitionId(draft, name);
  return {
    draft: { ...draft, transitions: [...draft.transitions, { ...source, id: copyId, name }] },
    id: copyId,
  };
}

export function removeTransition(draft: WorkflowDraft, id: string): WorkflowDraft {
  return { ...draft, transitions: draft.transitions.filter((transition) => transition.id !== id) };
}

export function updateTransition(
  draft: WorkflowDraft,
  id: string,
  patch: Partial<Omit<WorkflowTransition, 'id'>>,
): WorkflowDraft {
  return {
    ...draft,
    transitions: draft.transitions.map((transition) =>
      transition.id === id ? { ...transition, ...patch } : transition,
    ),
  };
}

/** Number of conditions, validators and post-functions of a transition. */
export function ruleCounts(transition: WorkflowTransition): {
  conditions: number;
  validators: number;
  postFunctions: number;
} {
  return {
    conditions: transition.conditions.length,
    validators: transition.validators.length + transition.requiredFields.length,
    postFunctions: transition.postFunctions.length,
  };
}

export function categoryOrder(category: WorkflowCategory): number {
  return WORKFLOW_CATEGORIES.indexOf(category);
}

export function sameRule(a: WorkflowRule, b: WorkflowRule): boolean {
  return a.type === b.type && JSON.stringify(a.params) === JSON.stringify(b.params);
}
