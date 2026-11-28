export type RoadmapGroupBy = 'epic' | 'version' | 'sprint';
export type RoadmapFilter = 'all' | 'open' | 'done';
export type RoadmapBarKind = 'epic' | 'version' | 'sprint';
export type RoadmapDateSource = 'own' | 'derived' | 'none';
export type RoadmapStatusCategory = 'todo' | 'in_progress' | 'done';

export interface RoadmapProgress {
  readonly total: number;
  readonly todo: number;
  readonly inProgress: number;
  readonly done: number;
  /** 0 to 100. */
  readonly percent: number;
  readonly pointsTotal: number;
  readonly pointsDone: number;
}

/** One bar of the roadmap: an epic, a version or a sprint. Dates are ISO days (YYYY-MM-DD). */
export interface RoadmapBar {
  readonly id: string;
  readonly kind: RoadmapBarKind;
  readonly key: string;
  readonly title: string;
  readonly status: string;
  readonly statusCategory: RoadmapStatusCategory;
  readonly assignee: string;
  readonly start: string | null;
  readonly end: string | null;
  readonly startSource: RoadmapDateSource;
  readonly endSource: RoadmapDateSource;
  readonly dated: boolean;
  readonly complete: boolean;
  readonly progress: RoadmapProgress;
}

export interface RoadmapDependency {
  readonly from: string;
  readonly to: string;
  readonly kind: string;
}

export interface Roadmap {
  readonly from: string | null;
  readonly to: string | null;
  readonly groupBy: RoadmapGroupBy;
  readonly bars: readonly RoadmapBar[];
  readonly versions: readonly RoadmapBar[];
  readonly sprints: readonly RoadmapBar[];
  readonly dependencies: readonly RoadmapDependency[];
}

export interface RoadmapQuery {
  readonly from?: string;
  readonly to?: string;
  readonly groupBy?: RoadmapGroupBy;
  readonly filter?: RoadmapFilter;
  readonly q?: string;
}
