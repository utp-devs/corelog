import {
  booleanAttribute,
  ChangeDetectionStrategy,
  Component,
  computed,
  contentChild,
  Directive,
  input,
} from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';

export type SectionHeadingLevel = 2 | 3 | 4;

let nextSectionId = 0;

/** Marks the element projected into the actions area of a section header (buttons, links). */
@Directive({
  selector: '[appSectionActions]',
})
export class SectionActions {}

/** Marks the element projected into the footer row of a section (totals, pagination, submit). */
@Directive({
  selector: '[appSectionFooter]',
})
export class SectionFooter {}

/**
 * Titled card block: header (title, description, actions), body and optional footer.
 * Actions are projected through an element with the appSectionActions attribute, the footer through
 * an element with the appSectionFooter attribute. Other content goes into the body.
 */
@Component({
  selector: 'app-section',
  imports: [NgTemplateOutlet],
  templateUrl: './section.html',
  styleUrl: './section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Section {
  /** Heading of the section. */
  readonly title = input<string>();
  /** Short text under the heading. */
  readonly description = input<string>();
  /** Body padding (default true). */
  readonly padded = input(true, { transform: booleanAttribute });
  /** No body padding at all, for tables and lists that draw their own edges. */
  readonly flush = input(false, { transform: booleanAttribute });
  /** Grows to fill the remaining height of a flex column. Default false. */
  readonly fill = input(false, { transform: booleanAttribute });
  /** Heading level of the title (default 2). */
  readonly headingLevel = input<SectionHeadingLevel>(2);
  /** Accessible name when there is no visible title. */
  readonly ariaLabel = input<string>();

  protected readonly titleId = `cl-section-title-${++nextSectionId}`;

  private readonly actionsSlot = contentChild(SectionActions);
  private readonly footerSlot = contentChild(SectionFooter);

  protected readonly hasActions = computed(() => this.actionsSlot() !== undefined);
  protected readonly hasFooter = computed(() => this.footerSlot() !== undefined);
  protected readonly hasHeader = computed(
    () => this.title() !== undefined || this.description() !== undefined || this.hasActions(),
  );
  protected readonly bodyPadded = computed(() => this.padded() && !this.flush());
}
