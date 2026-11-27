import { CatalogItem, CatalogKind } from '@corelog/core';

/** Spanish wording of one catalog. */
export interface CatalogCopy {
  readonly kind: CatalogKind;
  /** Tab label and table title. */
  readonly title: string;
  readonly intro: string;
  /** Singular noun with article for sentences ("el tipo de incidencia"). */
  readonly one: string;
  /** Singular noun without article, used in titles ("Nuevo tipo de incidencia"). */
  readonly oneBare: string;
  readonly createLabel: string;
  readonly newTitle: string;
  readonly editTitle: string;
  readonly emptyTitle: string;
  /** The catalog items carry an icon and a color. */
  readonly hasIcon: boolean;
  readonly nameMax: number;
}

export const CATALOG_COPY: Readonly<Record<CatalogKind, CatalogCopy>> = {
  'issue-types': {
    kind: 'issue-types',
    title: 'Tipos de incidencia',
    intro:
      'Los tipos clasifican el trabajo: tareas, errores, historias. La épica y la subtarea son obligatorias y no se pueden eliminar.',
    one: 'el tipo de incidencia',
    oneBare: 'tipo de incidencia',
    createLabel: 'Nuevo tipo',
    newTitle: 'Nuevo tipo de incidencia',
    editTitle: 'Editar tipo de incidencia',
    emptyTitle: 'Aún no hay tipos de incidencia',
    hasIcon: true,
    nameMax: 60,
  },
  priorities: {
    kind: 'priorities',
    title: 'Prioridades',
    intro:
      'Las prioridades indican la urgencia. Arrastra las filas para ordenarlas de la más urgente a la menos urgente.',
    one: 'la prioridad',
    oneBare: 'prioridad',
    createLabel: 'Nueva prioridad',
    newTitle: 'Nueva prioridad',
    editTitle: 'Editar prioridad',
    emptyTitle: 'Aún no hay prioridades',
    hasIcon: true,
    nameMax: 40,
  },
  resolutions: {
    kind: 'resolutions',
    title: 'Resoluciones',
    intro:
      'La resolución explica cómo terminó una incidencia: hecha, descartada, duplicada. Se asigna al resolverla.',
    one: 'la resolución',
    oneBare: 'resolución',
    createLabel: 'Nueva resolución',
    newTitle: 'Nueva resolución',
    editTitle: 'Editar resolución',
    emptyTitle: 'Aún no hay resoluciones',
    hasIcon: false,
    nameMax: 60,
  },
};

export const CATALOG_TABS: readonly CatalogCopy[] = [
  CATALOG_COPY['issue-types'],
  CATALOG_COPY.priorities,
  CATALOG_COPY.resolutions,
];

/** "3 incidencias", "1 incidencia", "Sin incidencias". */
export function usageText(count: number): string {
  if (count === 0) return 'Sin incidencias';
  return count === 1 ? '1 incidencia' : `${count} incidencias`;
}

/** Hierarchy label shown next to protected types, or null for standard types. */
export function hierarchyText(item: Pick<CatalogItem, 'hierarchyLevel'>): string | null {
  if (item.hierarchyLevel === 1) return 'Nivel épica';
  if (item.hierarchyLevel === -1) return 'Nivel subtarea';
  return null;
}

/** Why an item cannot be deleted, or null when it can be tried. */
export function protectedReason(
  kind: CatalogKind,
  item: CatalogItem,
  total: number,
): string | null {
  if (kind === 'issue-types' && item.hierarchyLevel === 1) {
    return 'La épica es obligatoria y no se puede eliminar.';
  }
  if (kind === 'issue-types' && item.hierarchyLevel === -1) {
    return 'La subtarea es obligatoria y no se puede eliminar.';
  }
  if (total <= 1) return 'Es el único elemento del catálogo.';
  return null;
}
