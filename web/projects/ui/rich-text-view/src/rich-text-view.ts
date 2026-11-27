import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  ViewEncapsulation,
  afterNextRender,
  effect,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { Router } from '@angular/router';
import { renderMarkdownFragment, type IssueResolver } from '@corelog/core';

interface LightboxImage {
  readonly src: string;
  readonly alt: string;
}

/**
 * Read-only view of Markdown (GFM plus mentions and issue keys). The source is parsed and
 * sanitized by the core Markdown pipeline and inserted as DOM nodes, never as raw HTML.
 */
@Component({
  selector: 'app-rich-text-view',
  templateUrl: './rich-text-view.html',
  styleUrl: './rich-text-view.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  encapsulation: ViewEncapsulation.None,
  host: {
    class: 'rich-text-view',
    'data-testid': 'rich-text-view',
    '(click)': 'onClick($event)',
  },
})
export class RichTextView {
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly content = viewChild.required<ElementRef<HTMLElement>>('content');
  private readonly closeButton = viewChild<ElementRef<HTMLButtonElement>>('closeButton');

  /** Markdown source (GFM). Raw HTML is not rendered. */
  readonly value = input.required<string>();
  /** Resolves an issue key to a link; undefined keeps the key as plain text. */
  readonly resolveIssue = input<IssueResolver>();
  /** When true, mention chips are buttons and emit mentionClick with the user id. */
  readonly mentionsClickable = input(false);
  /** Emits the user id when a mention button is activated. */
  readonly mentionClick = output<string>();

  protected readonly lightbox = signal<LightboxImage | null>(null);

  constructor() {
    effect(() => {
      const body = this.content().nativeElement;
      const fragment = renderMarkdownFragment(this.value(), {
        resolveIssue: this.resolveIssue(),
        interactiveMentions: this.mentionsClickable(),
      });
      body.replaceChildren(fragment);
    });
  }

  protected onClick(event: MouseEvent): void {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const imageButton = target.closest('[data-rt-image]');
    if (imageButton) {
      event.preventDefault();
      const img = imageButton.querySelector('img');
      this.openLightbox({
        src: img?.getAttribute('src') ?? '',
        alt: img?.getAttribute('alt') ?? '',
      });
      return;
    }

    const issueLink = target.closest('a[data-issue-key]');
    if (issueLink) {
      const href = issueLink.getAttribute('href') ?? '';
      if (href.startsWith('/') && !href.startsWith('//')) {
        event.preventDefault();
        void this.router.navigateByUrl(href);
      }
      return;
    }

    const mention = target.closest('button[data-user-id]');
    if (mention && this.mentionsClickable()) {
      const userId = mention.getAttribute('data-user-id');
      if (userId) this.mentionClick.emit(userId);
    }
  }

  protected closeLightbox(): void {
    this.lightbox.set(null);
  }

  protected onBackdropClick(event: MouseEvent): void {
    if (event.target === event.currentTarget) this.closeLightbox();
  }

  private openLightbox(image: LightboxImage): void {
    this.lightbox.set(image);
    afterNextRender(() => this.closeButton()?.nativeElement.focus(), { injector: this.injector });
  }
}
