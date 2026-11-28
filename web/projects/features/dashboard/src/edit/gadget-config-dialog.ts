import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import {
  AsyncState,
  FilterApiService,
  GadgetConfig,
  GadgetDefinition,
  GadgetField,
  GadgetType,
  ProjectApiService,
  gadgetDefinition,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Textarea } from 'primeng/textarea';
import { map } from 'rxjs';

/** What the person chose in the configuration dialog of a gadget. */
export interface GadgetConfigValue {
  readonly title: string;
  readonly config: GadgetConfig;
}

type FieldValue = string | number | boolean | string[] | null;

const MAX_TITLE = 120;

/**
 * Form with the configuration of one gadget: its title and the fields its type needs. The owner of the
 * dialog applies the value; the API validates it again when the dashboard is saved.
 */
@Component({
  selector: 'app-gadget-config-dialog',
  imports: [
    Button,
    Dialog,
    FieldControl,
    FieldRow,
    InputText,
    ReactiveFormsModule,
    SelectField,
    Textarea,
  ],
  templateUrl: './gadget-config-dialog.html',
  styleUrl: './gadget-config-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetConfigDialog {
  private readonly projectApi = inject(ProjectApiService);
  private readonly filterApi = inject(FilterApiService);
  private readonly projects = new AsyncState<SelectFieldOption[]>();
  private readonly filters = new AsyncState<SelectFieldOption[]>();

  readonly open = input(false);
  readonly mode = input<'add' | 'edit'>('add');
  readonly type = input<GadgetType | null>(null);
  readonly title = input('');
  readonly config = input<GadgetConfig>({});

  readonly applied = output<GadgetConfigValue>();
  readonly closeRequested = output<void>();

  protected readonly titleMax = MAX_TITLE;
  protected readonly definition = computed<GadgetDefinition | null>(() => {
    const type = this.type();
    return type ? gadgetDefinition(type) : null;
  });
  protected readonly heading = computed(() => {
    const label = this.definition()?.label ?? 'gadget';
    return this.mode() === 'add' ? `Añadir: ${label}` : `Configurar: ${label}`;
  });

  /** Controls are rebuilt for each gadget type: `__title` plus one control per configuration field. */
  protected readonly form = new FormGroup({
    __title: new FormControl<FieldValue>(''),
  }) as FormGroup;
  private readonly errorState = signal<Record<string, string>>({});
  private readonly submitted = signal(false);

  protected readonly projectOptions = computed(() => this.projects.value() ?? []);
  protected readonly filterOptions = computed(() => this.filters.value() ?? []);

  constructor() {
    effect(() => {
      const open = this.open();
      const definition = this.definition();
      const title = this.title();
      const config = this.config();
      untracked(() => {
        if (open && definition) this.buildForm(definition, title, config);
      });
    });
  }

  protected optionsOf(field: GadgetField): readonly SelectFieldOption[] {
    if (field.kind === 'project') return this.projectOptions();
    if (field.kind === 'filter') return this.filterOptions();
    return field.options ?? [];
  }

  protected isChecked(key: string, value: string): boolean {
    const current = this.form.controls[key]?.value;
    return Array.isArray(current) && current.includes(value);
  }

  protected toggleOption(key: string, value: string, event: Event): void {
    const checked = (event.target as HTMLInputElement).checked;
    const control = this.form.controls[key];
    const current = Array.isArray(control.value) ? control.value : [];
    control.setValue(checked ? [...current, value] : current.filter((entry) => entry !== value));
  }

  protected errorOf(key: string): string | undefined {
    return this.submitted() ? this.errorState()[key] : undefined;
  }

  protected submit(): void {
    const definition = this.definition();
    if (!definition) return;
    this.submitted.set(true);
    const problems = this.validate(definition);
    this.errorState.set(problems);
    if (Object.keys(problems).length > 0) return;
    const config: Record<string, unknown> = {};
    for (const field of definition.fields) {
      const value = this.form.controls[field.key]?.value;
      config[field.key] = field.kind === 'number' ? Number(value) : value;
    }
    this.applied.emit({ title: String(this.form.controls['__title'].value ?? '').trim(), config });
  }

  private buildForm(definition: GadgetDefinition, title: string, config: GadgetConfig): void {
    this.submitted.set(false);
    this.errorState.set({});
    const controls: Record<string, FormControl<FieldValue>> = {
      __title: new FormControl<FieldValue>(title || definition.label),
    };
    const merged = { ...definition.defaultConfig, ...config };
    for (const field of definition.fields) {
      const raw = merged[field.key];
      let value: FieldValue;
      if (field.kind === 'boolean') value = raw === true;
      else if (field.kind === 'multi') value = Array.isArray(raw) ? (raw as string[]) : [];
      else if (field.kind === 'number') value = typeof raw === 'number' ? raw : null;
      else value = raw === null || raw === undefined ? '' : String(raw);
      controls[field.key] = new FormControl<FieldValue>(value);
    }
    for (const name of Object.keys(this.form.controls)) this.form.removeControl(name);
    for (const [name, control] of Object.entries(controls)) this.form.addControl(name, control);
    if (definition.fields.some((field) => field.kind === 'project')) this.loadProjects();
    if (definition.fields.some((field) => field.kind === 'filter')) this.loadFilters();
  }

  private loadProjects(): void {
    if (this.projects.hasValue()) return;
    this.projects.load(
      this.projectApi.list().pipe(
        map((list) =>
          list.map((project) => ({
            value: project.id,
            label: `${project.name} (${project.key})`,
          })),
        ),
      ),
    );
  }

  private loadFilters(): void {
    if (this.filters.hasValue()) return;
    this.filters.load(
      this.filterApi
        .list('all')
        .pipe(map((list) => list.map((filter) => ({ value: filter.id, label: filter.name })))),
    );
  }

  private validate(definition: GadgetDefinition): Record<string, string> {
    const problems: Record<string, string> = {};
    const title = String(this.form.controls['__title'].value ?? '');
    if (title.length > MAX_TITLE) problems['__title'] = `Máximo ${MAX_TITLE} caracteres.`;
    for (const field of definition.fields) {
      const value = this.form.controls[field.key]?.value;
      if (field.kind === 'number') {
        const number = Number(value);
        const out =
          value === null ||
          value === '' ||
          !Number.isInteger(number) ||
          number < (field.min ?? 0) ||
          number > (field.max ?? Number.MAX_SAFE_INTEGER);
        if (out) problems[field.key] = field.help ?? 'Indica un número válido.';
      }
    }
    if (definition.type === 'filter_results') {
      const filter = this.form.controls['filter_id']?.value;
      const query = String(this.form.controls['query']?.value ?? '').trim();
      if (!filter && !query) problems['query'] = 'Elige un filtro guardado o escribe una consulta.';
    }
    if (definition.type === 'two_dimensional') {
      const x = this.form.controls['x_axis']?.value;
      const y = this.form.controls['y_axis']?.value;
      if (x && x === y) problems['y_axis'] = 'Las filas y las columnas deben ser campos distintos.';
    }
    return problems;
  }
}
