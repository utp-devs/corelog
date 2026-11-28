import {
  CdkConnectedOverlay,
  CdkOverlayOrigin,
  Overlay,
  type ConnectedPosition,
} from '@angular/cdk/overlay';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  TemplateRef,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';

import { RichTextView } from '@corelog/ui/rich-text-view';
import { UserAvatar } from '@corelog/ui/user-avatar';

export interface CommentReactionView {
  readonly key: string;
  readonly glyph: string;
  readonly label: string;
  readonly count: number;
  readonly viewerReacted: boolean;
}

/** One comment as the thread shows it. Text for the time is already formatted by the owner. */
export interface CommentView {
  readonly id: string;
  readonly authorId: string;
  readonly authorName: string;
  readonly body: string;
  readonly timeText: string;
  readonly timeTitle: string;
  readonly edited: boolean;
  readonly deleted: boolean;
  /** Visible only to people with the minimum role: shown with a badge. */
  readonly internal: boolean;
  readonly reactions: readonly CommentReactionView[];
  readonly canEdit: boolean;
  readonly canDelete: boolean;
}

export interface CommentThreadView extends CommentView {
  readonly replies: readonly CommentView[];
}

export interface CommentReactionEvent {
  readonly commentId: string;
  readonly key: string;
}

/** Every reaction the thread offers, to build the picker. */
export interface ReactionChoice {
  readonly key: string;
  readonly glyph: string;
  readonly label: string;
}

/** Gap in px between the reaction button and its picker. */
const PICKER_GAP = 4;

/** Picker below the button, or above it when there is no room below. */
const PICKER_POSITIONS: ConnectedPosition[] = [
  { originX: 'start', originY: 'bottom', overlayX: 'start', overlayY: 'top', offsetY: PICKER_GAP },
  { originX: 'start', originY: 'top', overlayX: 'start', overlayY: 'bottom', offsetY: -PICKER_GAP },
];

/**
 * One comment thread: the root comment and its replies (one level), with reactions, actions and the
 * edit marker. The editor of a comment being edited and the reply composer are rendered by the owner
 * through the `editor` and `composer` templates, so this component holds no form state.
 */
@Component({
  selector: 'app-comment-thread',
  imports: [NgTemplateOutlet, RichTextView, UserAvatar, CdkConnectedOverlay, CdkOverlayOrigin],
  templateUrl: './comment-thread.html',
  styleUrl: './comment-thread.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CommentThread {
  readonly thread = input.required<CommentThreadView>();
  readonly reactionChoices = input<readonly ReactionChoice[]>([]);
  /** Id of the comment whose editor is open, if any. */
  readonly editingId = input<string | null>(null);
  /** Id of the thread whose reply composer is open, if any. */
  readonly replyingId = input<string | null>(null);
  /** Template for the editor of the comment being edited; its context is the comment. */
  readonly editor = input<TemplateRef<{ $implicit: CommentView }>>();
  /** Template for the reply composer. */
  readonly composer = input<TemplateRef<{ $implicit: CommentThreadView }>>();
  readonly canReply = input(true);
  readonly canReact = input(true);

  readonly react = output<CommentReactionEvent>();
  readonly edit = output<string>();
  readonly remove = output<string>();
  readonly reply = output<string>();
  readonly history = output<string>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  protected readonly pickerFor = signal<string | null>(null);
  protected readonly positions = PICKER_POSITIONS;
  /** The picker closes when the page scrolls under it, so it never floats away from its button. */
  protected readonly scrollStrategy = inject(Overlay).scrollStrategies.close();

  protected toggleReaction(commentId: string, key: string): void {
    this.react.emit({ commentId, key });
  }

  protected togglePicker(commentId: string): void {
    this.pickerFor.update((current) => (current === commentId ? null : commentId));
  }

  protected choose(commentId: string, key: string): void {
    this.closePicker(true);
    this.react.emit({ commentId, key });
  }

  /** Closes the picker; the focus goes back to the button that opened it when asked. */
  protected closePicker(restoreFocus: boolean): void {
    const id = this.pickerFor();
    if (id === null) return;
    this.pickerFor.set(null);
    if (restoreFocus) {
      Array.from(this.host.nativeElement.querySelectorAll<HTMLElement>('[data-picker-trigger]'))
        .find((trigger) => trigger.dataset['pickerTrigger'] === id)
        ?.focus();
    }
  }

  /** A press outside the picker closes it; the button itself toggles. */
  protected onOutsideClick(event: MouseEvent): void {
    if (event.target instanceof Element && event.target.closest('[data-picker-trigger]')) return;
    this.closePicker(false);
  }

  protected focusFirst(overlay: CdkConnectedOverlay): void {
    queueMicrotask(() => overlay.overlayRef?.overlayElement.querySelector('button')?.focus());
  }

  /** Escape closes and returns the focus; the arrows move between the reactions. */
  protected onPickerKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      this.closePicker(true);
      return;
    }
    const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
    if (step === 0) return;
    const buttons = Array.from(
      (event.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button'),
    );
    const at = buttons.indexOf(document.activeElement as HTMLButtonElement);
    event.preventDefault();
    buttons[(at + step + buttons.length) % buttons.length]?.focus();
  }
}
