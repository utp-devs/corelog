import { TestBed } from '@angular/core/testing';

import { CommentThread, CommentThreadView, CommentView } from './comment-thread';

const CHOICES = [
  { key: 'thumbs_up', glyph: 'A', label: 'Me gusta' },
  { key: 'heart', glyph: 'B', label: 'Me encanta' },
];

function comment(overrides: Partial<CommentView> = {}): CommentView {
  return {
    id: 'c1',
    authorId: 'u1',
    authorName: 'Ana Torres',
    body: 'Hola equipo',
    timeText: 'hace 1 hora',
    timeTitle: '27 de noviembre',
    edited: false,
    deleted: false,
    internal: false,
    reactions: [],
    canEdit: false,
    canDelete: false,
    ...overrides,
  };
}

function mount(thread: CommentThreadView): {
  el: HTMLElement;
  events: Record<string, unknown[]>;
} {
  const fixture = TestBed.createComponent(CommentThread);
  fixture.componentRef.setInput('thread', thread);
  fixture.componentRef.setInput('reactionChoices', CHOICES);
  const events: Record<string, unknown[]> = { react: [], edit: [], remove: [], reply: [] };
  const component = fixture.componentInstance;
  component.react.subscribe((value) => events['react'].push(value));
  component.edit.subscribe((value) => events['edit'].push(value));
  component.remove.subscribe((value) => events['remove'].push(value));
  component.reply.subscribe((value) => events['reply'].push(value));
  fixture.detectChanges();
  return { el: fixture.nativeElement as HTMLElement, events };
}

function button(el: HTMLElement, text: string): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll('button')).find((item) =>
    item.textContent?.trim().startsWith(text),
  );
}

describe('CommentThread', () => {
  it('shows author, body, replies and the internal badge', () => {
    const { el } = mount({
      ...comment({ internal: true }),
      replies: [comment({ id: 'c2', authorName: 'Luis', body: 'Respuesta' })],
    });
    expect(el.textContent).toContain('Ana Torres');
    expect(el.textContent).toContain('Respuesta');
    expect(el.querySelector('[data-testid="comment-internal"]')).not.toBeNull();
    expect(el.querySelectorAll('[data-testid="comment"]')).toHaveLength(2);
  });

  it('replaces a deleted comment with a placeholder and offers no actions', () => {
    const { el } = mount({
      ...comment({ deleted: true, canEdit: true, canDelete: true }),
      replies: [],
    });
    expect(el.textContent).toContain('Comentario eliminado');
    expect(button(el, 'Editar')).toBeUndefined();
  });

  it('offers edit and delete only when the viewer may', () => {
    const { el } = mount({ ...comment(), replies: [] });
    expect(button(el, 'Editar')).toBeUndefined();
    expect(button(el, 'Eliminar')).toBeUndefined();
    const own = mount({ ...comment({ canEdit: true, canDelete: true }), replies: [] });
    button(own.el, 'Editar')?.click();
    button(own.el, 'Eliminar')?.click();
    expect(own.events['edit']).toEqual(['c1']);
    expect(own.events['remove']).toEqual(['c1']);
  });

  it('toggles a reaction and marks the ones the viewer already gave', () => {
    const { el, events } = mount({
      ...comment({
        reactions: [
          { key: 'heart', glyph: 'B', label: 'Me encanta', count: 2, viewerReacted: true },
        ],
      }),
      replies: [],
    });
    const mine = el.querySelector<HTMLButtonElement>('.comment-thread__reaction');
    expect(mine?.getAttribute('aria-pressed')).toBe('true');
    mine?.click();
    expect(events['react']).toEqual([{ commentId: 'c1', key: 'heart' }]);
  });

  it('opens the reaction picker and picks one', () => {
    const { el, events } = mount({ ...comment(), replies: [] });
    el.querySelector<HTMLButtonElement>('[aria-label="Añadir reacción"]')?.click();
    TestBed.tick();
    const choice = el.querySelector<HTMLButtonElement>('[aria-label="Me gusta"]');
    expect(choice).not.toBeNull();
    choice?.click();
    expect(events['react']).toEqual([{ commentId: 'c1', key: 'thumbs_up' }]);
  });

  it('asks to reply only on root comments', () => {
    const { el, events } = mount({
      ...comment(),
      replies: [comment({ id: 'c2' })],
    });
    const replyButtons = Array.from(el.querySelectorAll('button')).filter(
      (item) => item.textContent?.trim() === 'Responder',
    );
    expect(replyButtons).toHaveLength(1);
    replyButtons[0].click();
    expect(events['reply']).toEqual(['c1']);
  });
});
