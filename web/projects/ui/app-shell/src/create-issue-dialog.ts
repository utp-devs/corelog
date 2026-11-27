import {
  ChangeDetectionStrategy,
  Component,
  Injectable,
  effect,
  inject,
  model,
  signal,
  untracked,
} from '@angular/core';

import { CreateIssueForm } from './create-issue-form';

/**
 * Lets any part of the shell ask for the create-issue dialog (for example the 'c' shortcut) without
 * knowing where the dialog is mounted. Each call to `open` opens it once.
 */
@Injectable({ providedIn: 'root' })
export class CreateIssueLauncher {
  private readonly count = signal(0);

  /** Number of requests so far; the dialog opens whenever it grows. */
  readonly requests = this.count.asReadonly();

  open(): void {
    this.count.update((value) => value + 1);
  }
}

/**
 * Quick create-issue dialog of the top bar. The form (and the stores it needs) exists only while the
 * dialog is open.
 */
@Component({
  selector: 'app-create-issue-dialog',
  imports: [CreateIssueForm],
  templateUrl: './create-issue-dialog.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CreateIssueDialog {
  private readonly launcher = inject(CreateIssueLauncher);
  private seen = this.launcher.requests();

  /** Whether the dialog is shown. The dialog sets it to false when it closes itself. */
  readonly open = model(false);

  constructor() {
    effect(() => {
      const requests = this.launcher.requests();
      untracked(() => {
        if (requests !== this.seen) {
          this.seen = requests;
          this.open.set(true);
        }
      });
    });
  }

  protected close(): void {
    this.open.set(false);
  }
}
