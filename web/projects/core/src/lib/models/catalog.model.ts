/** The three configurable catalogs of an organization. The value is the API path segment. */
export type CatalogKind = 'issue-types' | 'priorities' | 'resolutions';

export const CATALOG_KINDS: readonly CatalogKind[] = ['issue-types', 'priorities', 'resolutions'];

/** Colors of the palette, referenced by key. The UI maps each key to its theme tokens. */
export const CATALOG_COLORS = [
  'gray',
  'red',
  'orange',
  'amber',
  'green',
  'teal',
  'blue',
  'indigo',
  'purple',
  'pink',
] as const;
export type CatalogColor = (typeof CATALOG_COLORS)[number];

/** Icons of the palette, referenced by key. The UI maps each key to a glyph. */
export const CATALOG_ICONS = [
  'task',
  'bug',
  'story',
  'epic',
  'subtask',
  'star',
  'flag',
  'bolt',
  'alert',
  'check',
  'layers',
  'tag',
  'book',
  'rocket',
  'shield',
  'clock',
  'circle',
  'arrow-up',
  'arrow-down',
  'minus',
  'chevrons-up',
  'chevrons-down',
] as const;
export type CatalogIcon = (typeof CATALOG_ICONS)[number];

/** 1 epic, 0 standard, -1 subtask. */
export type HierarchyLevel = -1 | 0 | 1;
export const HIERARCHY_EPIC: HierarchyLevel = 1;
export const HIERARCHY_STANDARD: HierarchyLevel = 0;
export const HIERARCHY_SUBTASK: HierarchyLevel = -1;

/** Item of a catalog with its position and how many issues use it. */
export interface CatalogItem {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly icon: string | null;
  readonly color: string | null;
  /** Only issue types carry a hierarchy level. */
  readonly hierarchyLevel: HierarchyLevel | null;
  readonly position: number;
  readonly isDefault: boolean;
  readonly usageCount: number;
  readonly createdAt: string;
  readonly updatedAt: string;
}

/** Issue type as embedded in an issue. */
export interface IssueTypeRef {
  readonly id: string;
  readonly name: string;
  readonly icon: string;
  readonly color: string;
  readonly hierarchyLevel: HierarchyLevel;
}

/** Priority as embedded in an issue. */
export interface PriorityRef {
  readonly id: string;
  readonly name: string;
  readonly icon: string;
  readonly color: string;
}

/** Resolution of a finished issue. */
export interface ResolutionRef {
  readonly id: string;
  readonly name: string;
}

export interface CatalogCreateInput {
  readonly name: string;
  readonly description?: string;
  readonly icon?: string;
  readonly color?: string;
  /** Only for issue types. Defaults to standard. */
  readonly hierarchyLevel?: HierarchyLevel;
  readonly isDefault?: boolean;
}

export interface CatalogPatchInput {
  readonly name?: string;
  readonly description?: string;
  readonly icon?: string;
  readonly color?: string;
  readonly isDefault?: boolean;
}

/** Issue types a project uses. `configured` is false when it uses every type of the organization. */
export interface ProjectIssueTypes {
  readonly items: readonly CatalogItem[];
  readonly configured: boolean;
}
