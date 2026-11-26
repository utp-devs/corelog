import { describe, expect, it } from 'vitest'

import { applyToolbarAction, extractIssueKeys, insertAtSelection, mentionToken, parseMentions, splitIssueKeys } from './index'

const UID = '3f2a9c1e-7b4d-4e8a-9c2f-1a2b3c4d5e6f'

describe('mentions', () => {
  it('parses mention tokens and keeps surrounding text in order', () => {
    const text = `Hola @[Ana Pérez](user:${UID}), revisa esto`
    expect(parseMentions(text)).toEqual([
      { type: 'text', value: 'Hola ' },
      { type: 'mention', userId: UID, name: 'Ana Pérez' },
      { type: 'text', value: ', revisa esto' },
    ])
  })

  it('leaves tokens with a non-UUID target as text', () => {
    expect(parseMentions('@[Ana](user:nope)')).toEqual([{ type: 'text', value: '@[Ana](user:nope)' }])
  })

  it('builds a token and strips characters that break the syntax', () => {
    expect(mentionToken({ id: UID, name: 'Ana [Admin] (x)' })).toBe(`@[Ana Admin x](user:${UID})`)
    expect(parseMentions(mentionToken({ id: UID, name: 'Ana' }))).toEqual([
      { type: 'mention', userId: UID, name: 'Ana' },
    ])
  })

  it('rejects an invalid user id when building a token', () => {
    expect(() => mentionToken({ id: 'x', name: 'Ana' })).toThrow(RangeError)
  })
})

describe('issue keys', () => {
  it('extracts unique keys in order of appearance', () => {
    expect(extractIssueKeys('Ver CL-12 y CL-7, luego CL-12 otra vez')).toEqual(['CL-12', 'CL-7'])
  })

  it('requires word boundaries and an uppercase project prefix', () => {
    expect(extractIssueKeys('xCL-1 cl-2 A-3 ABC-')).toEqual([])
    expect(extractIssueKeys('(PROJ2-99).')).toEqual(['PROJ2-99'])
  })

  it('splits text around keys', () => {
    expect(splitIssueKeys('a CL-1 b')).toEqual([
      { type: 'text', value: 'a ' },
      { type: 'key', key: 'CL-1' },
      { type: 'text', value: ' b' },
    ])
  })
})

describe('insertAtSelection', () => {
  it('wraps the selection and keeps it selected inside the markers', () => {
    expect(insertAtSelection('hola mundo', 5, 10, '**', '**')).toEqual({
      value: 'hola **mundo**',
      selectionStart: 7,
      selectionEnd: 12,
    })
  })

  it('places the caret between markers when nothing is selected', () => {
    expect(insertAtSelection('ab', 1, 1, '[', ']')).toEqual({ value: 'a[]b', selectionStart: 2, selectionEnd: 2 })
  })

  it('accepts a reversed range and clamps out-of-range offsets', () => {
    expect(insertAtSelection('abc', 3, 1, '_', '_').value).toBe('a_bc_')
    expect(insertAtSelection('abc', -5, 99, '', '').value).toBe('abc')
  })
})

describe('applyToolbarAction', () => {
  it('wraps bold, italic, strikethrough and inline code', () => {
    expect(applyToolbarAction('bold', 'x y', 2, 3).value).toBe('x **y**')
    expect(applyToolbarAction('italic', 'x y', 2, 3).value).toBe('x *y*')
    expect(applyToolbarAction('strikethrough', 'x y', 2, 3).value).toBe('x ~~y~~')
    expect(applyToolbarAction('code', 'x y', 2, 3).value).toBe('x `y`')
  })

  it('inserts a placeholder and selects it when nothing is selected', () => {
    const edit = applyToolbarAction('bold', '', 0, 0)
    expect(edit.value).toBe('**texto**')
    expect(edit.value.slice(edit.selectionStart, edit.selectionEnd)).toBe('texto')
  })

  it('inserts a link around the selection', () => {
    expect(applyToolbarAction('link', 'ver docs', 4, 8).value).toBe('ver [docs](https://)')
  })

  it('prefixes each selected line for lists, quotes and headings', () => {
    const text = 'uno\ndos\ntres'
    expect(applyToolbarAction('bulletList', text, 0, 7).value).toBe('- uno\n- dos\ntres')
    expect(applyToolbarAction('numberedList', text, 0, text.length).value).toBe('1. uno\n2. dos\n3. tres')
    expect(applyToolbarAction('taskList', 'tarea', 0, 0).value).toBe('- [ ] tarea')
    expect(applyToolbarAction('quote', 'cita', 0, 0).value).toBe('> cita')
    expect(applyToolbarAction('heading', 'titulo', 0, 0).value).toBe('## titulo')
  })

  it('wraps code blocks and inserts a table block', () => {
    expect(applyToolbarAction('codeBlock', 'npm i', 0, 5).value).toBe('```\nnpm i\n```\n')
    const table = applyToolbarAction('table', '', 0, 0).value
    expect(table.split('\n')[0]).toBe('| Columna 1 | Columna 2 |')
    expect(table).toContain('| --- | --- |')
  })
})
