import { ChangeDetectionStrategy, Component, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { parseInvitationToken } from '@corelog/core';

/** Wizard step 1: create an organization, or join one with an invitation link or token. */
@Component({
  selector: 'app-choose-step',
  imports: [Button, InputText, FormsModule],
  templateUrl: './choose-step.html',
  styleUrl: './choose-step.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChooseStep {
  private readonly router = inject(Router);

  /** Starts the organization form (step 2). */
  readonly create = output<void>();

  protected readonly link = signal('');
  protected readonly error = signal<string | null>(null);

  protected onLinkInput(value: string): void {
    this.link.set(value);
    this.error.set(null);
  }

  protected join(): void {
    const token = parseInvitationToken(this.link());
    if (!token) {
      this.error.set('Introduce un enlace de invitación válido.');
      return;
    }
    void this.router.navigateByUrl(`/invite/${encodeURIComponent(token)}`);
  }
}
