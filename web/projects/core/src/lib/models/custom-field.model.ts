import { CustomFieldOption, CustomFieldType } from './issue-view.model';

export interface CustomFieldContext {
  readonly projectId: string | null;
  readonly issueTypeId: string | null;
  readonly required: boolean;
  readonly defaultValue: unknown;
}

export interface CustomField {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly type: CustomFieldType;
  readonly options: readonly CustomFieldOption[];
  readonly min: number | null;
  readonly max: number | null;
  readonly position: number;
  readonly contexts: readonly CustomFieldContext[];
}

export interface CustomFieldOptionDraft {
  readonly id?: string;
  readonly label: string;
  readonly color?: string;
  readonly archived?: boolean;
}

export interface CreateCustomFieldInput {
  readonly name: string;
  readonly description?: string;
  readonly type: CustomFieldType;
  readonly options?: readonly { label: string; color?: string }[];
  readonly min?: number;
  readonly max?: number;
}

export interface PatchCustomFieldInput {
  readonly name?: string;
  readonly description?: string;
  readonly options?: readonly CustomFieldOptionDraft[];
  readonly min?: number | null;
  readonly max?: number | null;
}

/** One entry of the field layout of a project for an issue type. */
export interface FieldLayoutItem {
  readonly kind: 'system' | 'custom';
  /** System field key (summary, description, priority...). */
  readonly key: string | null;
  /** Custom field id. */
  readonly fieldId: string | null;
  readonly name: string;
  readonly type: string;
  readonly options: readonly CustomFieldOption[];
  readonly required: boolean;
}

export interface FieldLayout {
  readonly issueTypeId: string;
  readonly configured: boolean;
  readonly items: readonly FieldLayoutItem[];
}

export interface FieldLayoutEntryInput {
  readonly kind: 'system' | 'custom';
  readonly key?: string;
  readonly fieldId?: string;
  readonly required: boolean;
}
