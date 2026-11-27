import { ChangeDetectionStrategy, Component, inject, model } from '@angular/core';
import { Router } from '@angular/router';
import { PrimeTemplate } from 'primeng/api';
import { Dialog } from 'primeng/dialog';

/**
 * Placeholder for the quick create-issue dialog. It points to the full issue form until the quick
 * flow exists.
 */
@Component({
  selector: 'app-create-issue-dialog',
  imports: [Dialog, PrimeTemplate],
  templateUrl: './create-issue-dialog.html',
  styleUrl: './create-issue-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CreateIssueDialog {
  private readonly router = inject(Router);

  /** Whether the dialog is shown. The dialog sets it to false when it closes itself. */
  readonly open = model(false);

  protected close(): void {
    this.open.set(false);
  }

  protected goToForm(): void {
    this.close();
    void this.router.navigateByUrl('/issues/new');
  }
}
