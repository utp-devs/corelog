import { Signal, computed, effect, inject, signal, untracked } from '@angular/core';
import { FormControl } from '@angular/forms';
import { Subscription } from 'rxjs';

import { CatalogItem, HIERARCHY_STANDARD, HIERARCHY_SUBTASK } from '../models/catalog.model';
import { CatalogStore } from '../state/catalog.store';

/** Options of the type and priority pickers of an issue form, loaded from the organization catalogs. */
export interface IssueCatalogOptions {
  /** Types offered for a new issue: the ones the project uses, without subtasks (they need a parent). */
  readonly typeOptions: Signal<CatalogItem[]>;
  readonly priorityOptions: Signal<CatalogItem[]>;
  /** Id of the type preselected for new issues, or empty while nothing is loaded. */
  readonly defaultTypeId: Signal<string>;
  readonly defaultPriorityId: Signal<string>;
}

/**
 * Builds the picker options for the project in `projectId` and keeps the given controls on a valid
 * value: an empty control, or one whose item is no longer offered, takes the default. Must run in an
 * injection context (a component constructor or field initializer).
 */
export function injectIssueCatalogOptions(
  projectId: () => string | null | undefined,
  controls?: {
    readonly type?: FormControl<string>;
    readonly priority?: FormControl<string>;
  },
  settings: { readonly allowSubtasks?: boolean } = {},
): IssueCatalogOptions {
  const store = inject(CatalogStore);
  const projectTypes = signal<CatalogItem[] | null>(null);
  let subscription: Subscription | null = null;

  store.ensureLoaded();

  effect((onCleanup) => {
    const id = projectId();
    untracked(() => {
      subscription?.unsubscribe();
      projectTypes.set(null);
      if (!id) return;
      subscription = store.projectIssueTypes(id).subscribe({
        next: (result) => projectTypes.set([...result.items]),
        error: () => projectTypes.set(null),
      });
    });
    onCleanup(() => subscription?.unsubscribe());
  });

  const typeOptions = computed(() =>
    (projectTypes() ?? store.issueTypes()).filter(
      (item) => settings.allowSubtasks || item.hierarchyLevel !== HIERARCHY_SUBTASK,
    ),
  );
  const priorityOptions = computed(() => store.priorities());
  const defaultTypeId = computed(() => {
    const options = typeOptions();
    return (
      (
        options.find((item) => item.isDefault) ??
        options.find((item) => item.hierarchyLevel === HIERARCHY_STANDARD) ??
        options[0]
      )?.id ?? ''
    );
  });
  const defaultPriorityId = computed(() => store.defaultPriority()?.id ?? '');

  const keepValid = (
    control: FormControl<string> | undefined,
    options: Signal<CatalogItem[]>,
    fallback: Signal<string>,
  ): void => {
    if (!control) return;
    effect(() => {
      const items = options();
      const next = fallback();
      untracked(() => {
        if (items.length > 0 && !items.some((item) => item.id === control.value)) {
          control.setValue(next);
        }
      });
    });
  };
  keepValid(controls?.type, typeOptions, defaultTypeId);
  keepValid(controls?.priority, priorityOptions, defaultPriorityId);

  return { typeOptions, priorityOptions, defaultTypeId, defaultPriorityId };
}
