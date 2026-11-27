import { CatalogItem } from '../models/catalog.model';
import {
  findPriorityByLegacyKey,
  findTypeByLegacyKey,
  issueTypeDisplayOf,
  issueTypeRefFromKey,
  legacyPriorityKeyOf,
  legacyTypeKeyOf,
  priorityDisplayOf,
  priorityRefFromKey,
  toCatalogOption,
  toIssueTypeRef,
  toPriorityRef,
} from './catalog-refs';

function item(name: string, extra: Partial<CatalogItem> = {}): CatalogItem {
  return {
    id: `id-${name}`,
    name,
    description: '',
    icon: 'tag',
    color: 'green',
    hierarchyLevel: 0,
    position: 0,
    isDefault: false,
    usageCount: 0,
    createdAt: '',
    updatedAt: '',
    ...extra,
  };
}

describe('catalog refs', () => {
  it('shows legacy board keys with the palette of the default catalogs', () => {
    expect(issueTypeRefFromKey('bug')).toMatchObject({ name: 'Error', icon: 'bug', color: 'red' });
    expect(priorityRefFromKey('critical')).toMatchObject({ name: 'Máxima', icon: 'chevrons-up' });
    expect(issueTypeRefFromKey('other')).toBeUndefined();
    expect(priorityRefFromKey(undefined)).toBeUndefined();
  });

  it('falls back to a neutral entry, always the same object for the same key', () => {
    const first = priorityDisplayOf('urgente');
    expect(first).toMatchObject({ name: 'urgente', color: 'gray' });
    expect(priorityDisplayOf('urgente')).toBe(first);
    expect(issueTypeDisplayOf('raro').icon).toBe('circle');
    expect(issueTypeDisplayOf('task').name).toBe('Tarea');
  });

  it('maps a catalog item to the legacy filter key', () => {
    expect(legacyTypeKeyOf(item('Cualquier cosa', { hierarchyLevel: 1 }))).toBe('epic');
    expect(legacyTypeKeyOf(item('Algo', { hierarchyLevel: -1 }))).toBe('subtask');
    expect(legacyTypeKeyOf(item('Tarea'))).toBe('task');
    expect(legacyTypeKeyOf(item('Épica', { hierarchyLevel: 0 }))).toBeUndefined();
    expect(legacyTypeKeyOf(item('Hotfix'))).toBeUndefined();
    expect(legacyPriorityKeyOf(item('Crítica'))).toBeUndefined();
    expect(legacyPriorityKeyOf(item('Máxima'))).toBe('critical');
    expect(legacyPriorityKeyOf(item('Mínima'))).toBeUndefined();
  });

  it('finds the item of a saved legacy key', () => {
    const types = [item('Tarea'), item('Error')];
    expect(findTypeByLegacyKey(types, 'bug')?.name).toBe('Error');
    expect(findPriorityByLegacyKey([item('Alta'), item('Baja')], 'low')?.name).toBe('Baja');
    expect(findPriorityByLegacyKey([item('Alta')], 'low')).toBeUndefined();
  });

  it('builds embedded refs and select options with palette defaults', () => {
    expect(toIssueTypeRef(item('X', { icon: null, color: null, hierarchyLevel: null }))).toEqual({
      id: 'id-X',
      name: 'X',
      icon: 'task',
      color: 'gray',
      hierarchyLevel: 0,
    });
    expect(toPriorityRef(item('P'))).toEqual({
      id: 'id-P',
      name: 'P',
      icon: 'tag',
      color: 'green',
    });
    expect(toCatalogOption(item('P'))).toEqual({
      value: 'id-P',
      label: 'P',
      icon: 'tag',
      color: 'green',
    });
  });
});
