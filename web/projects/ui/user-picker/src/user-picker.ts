import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  model,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { initialsOf } from '@corelog/ui/avatar';
import { AutoComplete } from 'primeng/autocomplete';

export interface PickableUser {
  id: string;
  name: string;
  email?: string;
}

interface PickerOption {
  id: string;
  name: string;
  /** Name followed by the email in parentheses when the user has one. */
  label: string;
  initials: string;
  searchText: string;
}

/**
 * User selector with search and an avatar with initials. Loading the directory is the parent's job: the
 * search output emits the typed text so the parent can query the server.
 */
@Component({
  selector: 'app-user-picker',
  imports: [FormsModule, AutoComplete],
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
  readonly disabled = input(false);

  protected readonly options = computed<PickerOption[]>(() =>
    this.users().map((user) => ({
      id: user.id,
      name: user.name,
      label: user.email ? `${user.name} (${user.email})` : user.name,
      initials: initialsOf(user.name),
      searchText: [user.name, user.email ?? ''].join(' '),
    })),
  );
  protected readonly emptyText = computed(() => (this.loading() ? '' : 'Sin resultados'));
  /** Text typed in the field; options are narrowed to the users matching it. */
  private readonly query = signal('');

  /** Options that match the typed text (case-insensitive, name or email). */
  protected readonly suggestions = computed<PickerOption[]>(() => {
    const term = this.query().trim().toLowerCase();
    return this.options().filter(
      (option) => term === '' || option.searchText.toLowerCase().includes(term),
    );
  });

  protected readonly selected = computed<PickerOption | null>(
    () => this.options().find((option) => option.id === this.value()) ?? null,
  );

  protected onComplete(term: string): void {
    this.query.set(term);
    this.search.emit(term);
  }

  /** Only a suggestion sets the value; typed text without a match clears the selection. */
  protected onSelect(selected: PickerOption | string | null | undefined): void {
    this.value.set(typeof selected === 'object' && selected !== null ? selected.id : undefined);
  }
}
