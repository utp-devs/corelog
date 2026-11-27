import { ChangeDetectionStrategy, Component, computed, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { initialsOf } from '@corelog/ui/avatar';
import { Select } from 'primeng/select';

export interface PickableUser {
  id: string;
  name: string;
  email?: string;
}

interface PickerOption {
  id: string;
  name: string;
  initials: string;
  searchText: string;
}

/**
 * User selector with search and an avatar with initials. Loading the directory is the parent's job: the
 * search output emits the typed text so the parent can query the server.
 */
@Component({
  selector: 'app-user-picker',
  imports: [FormsModule, Select],
  templateUrl: './user-picker.html',
  styleUrl: './user-picker.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class UserPicker {
  readonly users = input.required<PickableUser[]>();
  /** Selected user id. */
  readonly value = model<string | undefined>(undefined);
  /** Emits the text typed in the search field. */
  readonly search = output<string>();
  readonly loading = input(false);
  readonly placeholder = input('Buscar persona');
  readonly allowClear = input(true);

  protected readonly options = computed<PickerOption[]>(() =>
    this.users().map((user) => ({
      id: user.id,
      name: user.name,
      initials: initialsOf(user.name),
      searchText: [user.name, user.email ?? ''].join(' '),
    })),
  );
  protected readonly emptyText = computed(() => (this.loading() ? '' : 'Sin resultados'));

  protected onSelect(selected: string | null | undefined): void {
    this.value.set(selected ?? undefined);
  }
}
