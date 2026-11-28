import { GADGET_TYPES } from '../models/dashboard-v2.model';
import { GADGET_CATALOG, cleanGadgetConfig, gadgetDefinition } from './gadget-catalog';

describe('gadget catalog', () => {
  it('describes every gadget type once, with Spanish copy', () => {
    expect(GADGET_CATALOG.map((d) => d.type).sort()).toEqual([...GADGET_TYPES].sort());
    for (const definition of GADGET_CATALOG) {
      expect(definition.label).not.toBe('');
      expect(definition.description).not.toBe('');
      expect(definition.icon.startsWith('pi-')).toBe(true);
      for (const field of definition.fields) expect(field.label).not.toBe('');
    }
  });

  it('finds a definition by type', () => {
    expect(gadgetDefinition('pie').label).toBe('Gráfico circular');
  });

  it('drops empty optional values before sending', () => {
    expect(
      cleanGadgetConfig('issue_statistics', { group_by: 'status', project_id: null, limit: 5 }),
    ).toEqual({ group_by: 'status', limit: 5 });
    expect(cleanGadgetConfig('filter_results', { columns: [], query: '  x  ' })).toEqual({
      query: 'x',
    });
  });

  it('keeps one source for a filter gadget: the saved filter wins over the query', () => {
    expect(
      cleanGadgetConfig('filter_results', { filter_id: 'f-1', query: 'status = open' }),
    ).toEqual({ filter_id: 'f-1' });
  });

  it('keeps an empty text and an empty favorites type list', () => {
    expect(cleanGadgetConfig('text', { markdown: '' })).toEqual({ markdown: '' });
    expect(cleanGadgetConfig('favorites', { types: [], limit: 10 })).toEqual({
      types: [],
      limit: 10,
    });
  });
});
