import { describe, expect, it } from 'vitest';

import { WorkflowView } from '../models/workflow.model';
import {
  addStatus,
  addTransition,
  draftFromView,
  draftToInput,
  draftsEqual,
  duplicateTransition,
  moveStatus,
  removeStatus,
  removedStatuses,
  renameStatus,
  ruleCounts,
  setInitial,
  uniqueTransitionId,
  updateTransition,
} from './workflow-draft';

const VIEW: WorkflowView = {
  statuses: [
    { id: 's1', name: 'Por hacer', category: 'todo', position: 0, issueCount: 2 },
    { id: 's2', name: 'En curso', category: 'in_progress', position: 1, issueCount: 0 },
    { id: 's3', name: 'Hecho', category: 'done', position: 2, issueCount: 1 },
  ],
  initialStatusId: 's1',
  transitions: [
    {
      id: 'start',
      name: 'Iniciar',
      from: ['s1'],
      to: 's2',
      conditions: [],
      validators: [{ type: 'comment_required', params: {} }],
      postFunctions: [],
      requiredFields: ['due_date'],
    },
    {
      id: 'finish',
      name: 'Terminar',
      from: [],
      to: 's3',
      conditions: [],
      validators: [],
      postFunctions: [],
      requiredFields: [],
    },
  ],
  version: 3,
  updatedAt: '2026-11-27T00:00:00Z',
  isTemplateDefault: false,
};

describe('workflow draft', () => {
  it('uses the status id as ref and keeps the saved order', () => {
    const draft = draftFromView(VIEW);
    expect(draft.statuses.map((status) => status.ref)).toEqual(['s1', 's2', 's3']);
    expect(draft.initialRef).toBe('s1');
    expect(draftsEqual(draft, draftFromView(VIEW))).toBe(true);
  });

  it('sends saved statuses by id and new ones by name and category', () => {
    const draft = addStatus(draftFromView(VIEW), { name: 'Bloqueado', category: 'in_progress' });
    const input = draftToInput(draft, 3, { s1: 's2' });
    expect(input.statuses[0]).toEqual({ ref: 's1', id: 's1' });
    expect(input.statuses[3]).toEqual({ ref: 'new-1', name: 'Bloqueado', category: 'in_progress' });
    expect(input.expectedVersion).toBe(3);
    expect(input.statusMapping).toEqual({ s1: 's2' });
  });

  it('adds a catalog status once and gives new statuses unique refs', () => {
    let draft = draftFromView(VIEW);
    expect(addStatus(draft, { id: 's2', name: 'En curso', category: 'in_progress' })).toBe(draft);
    draft = addStatus(draft, { name: 'A', category: 'todo' });
    draft = addStatus(draft, { name: 'B', category: 'todo' });
    expect(draft.statuses.slice(3).map((status) => status.ref)).toEqual(['new-1', 'new-2']);
  });

  it('removes a status with the transitions into it and the sources from it', () => {
    const draft = removeStatus(draftFromView(VIEW), 's2');
    // "start" went into s2: gone. "finish" from any status stays.
    expect(draft.transitions.map((item) => item.id)).toEqual(['finish']);
    const first = removeStatus(draftFromView(VIEW), 's1');
    expect(first.initialRef).toBe('s2');
    // "start" only left from s1: it is dropped, not turned into "from any status".
    expect(first.transitions.map((item) => item.id)).toEqual(['finish']);
  });

  it('reports the saved statuses that left the draft', () => {
    const baseline = draftFromView(VIEW);
    const draft = removeStatus(baseline, 's3');
    expect(removedStatuses(baseline, draft).map((status) => status.id)).toEqual(['s3']);
  });

  it('renames, moves and marks the initial status', () => {
    let draft = renameStatus(draftFromView(VIEW), 's2', 'Trabajando');
    expect(draft.statuses[1].name).toBe('Trabajando');
    draft = moveStatus(draft, 0, 2);
    expect(draft.statuses.map((status) => status.ref)).toEqual(['s2', 's3', 's1']);
    expect(moveStatus(draft, 0, 9)).toBe(draft);
    draft = setInitial(draft, 's3');
    expect(draft.initialRef).toBe('s3');
    expect(setInitial(draft, 'nope')).toBe(draft);
  });

  it('creates, duplicates and edits transitions with unique ids', () => {
    let draft = draftFromView(VIEW);
    const added = addTransition(draft, { name: 'Iniciar', from: ['s1'], to: 's2' });
    expect(added.id).toBe('iniciar');
    draft = added.draft;
    expect(uniqueTransitionId(draft, 'Iniciar')).toBe('iniciar_2');
    const copy = duplicateTransition(draft, 'start');
    expect(copy?.draft.transitions.find((item) => item.id === copy.id)?.name).toBe(
      'Iniciar (copia)',
    );
    draft = updateTransition(draft, 'start', { to: 's3' });
    expect(draft.transitions[0].to).toBe('s3');
  });

  it('counts the rules of a transition, required fields included in the validators', () => {
    expect(ruleCounts(VIEW.transitions[0])).toEqual({
      conditions: 0,
      validators: 2,
      postFunctions: 0,
    });
  });
});
