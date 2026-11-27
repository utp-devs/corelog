import {
  ChangeDetectionStrategy,
  Component,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { WorkflowTemplate, WorkflowTemplateId } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

/** Lists the workflow templates and asks for confirmation before one replaces the project workflow. */
@Component({
  selector: 'app-workflow-templates-dialog',
  imports: [Button, Dialog, Message],
  templateUrl: './templates-dialog.html',
  styleUrl: './templates-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowTemplatesDialog {
  readonly open = input(false);
  readonly templates = input<readonly WorkflowTemplate[]>([]);
  readonly loading = input(false);
  readonly loadFailed = input(false);
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly applied = output<WorkflowTemplateId>();
  readonly closeRequested = output<void>();
  readonly retry = output<void>();

  protected readonly choice = signal<WorkflowTemplateId | null>(null);

  constructor() {
    effect(() => {
      if (this.open()) untracked(() => this.choice.set(null));
    });
  }

  protected summary(template: WorkflowTemplate): string {
    return template.statuses.map((status) => status.name).join(' → ');
  }

  protected confirm(): void {
    const id = this.choice();
    if (id && !this.busy()) this.applied.emit(id);
  }
}
