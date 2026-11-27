import { ChangeDetectionStrategy, Component } from '@angular/core';

import { ShellIcon } from './shell-icon';

/** Notes are pending future work: the button is visible but disabled so the header layout is final. */
@Component({
  selector: 'app-notes-button',
  imports: [ShellIcon],
  template: `<button
    type="button"
    class="notes"
    disabled
    aria-label="Notas (próximamente)"
    title="Próximamente"
  >
    <app-shell-icon name="edit" />
  </button>`,
  styleUrl: './notes-button.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotesButton {}
