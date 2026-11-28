/** Column templates of a dashboard: one to three equal columns, or two columns of 2:1 or 1:2. */
export type DashboardLayout = '1' | '2' | '3' | '2-1' | '1-2';

export const DASHBOARD_LAYOUTS: readonly DashboardLayout[] = ['1', '2', '3', '2-1', '1-2'];

export const GADGET_TYPES = [
  'assigned_to_me',
  'created_by_me',
  'filter_results',
  'issue_statistics',
  'pie',
  'two_dimensional',
  'created_vs_resolved',
  'sprint_burndown',
  'activity_stream',
  'recent_projects',
  'favorites',
  'text',
  'calendar_upcoming',
] as const;

export type GadgetType = (typeof GADGET_TYPES)[number];

/** Most gadgets a dashboard can hold. */
export const MAX_GADGETS = 20;

/** Configuration of a gadget: the keys depend on its type (see the gadget catalog). */
export type GadgetConfig = Readonly<Record<string, unknown>>;

export interface DashboardGadget {
  /** Identifier stored by the API. Gadgets added in the editor have a local `draft-` identifier. */
  readonly id: string;
  readonly type: GadgetType;
  readonly title: string;
  readonly config: GadgetConfig;
  readonly position: number;
  readonly columnIndex: number;
}

/** What the viewer can do with a dashboard. */
export type DashboardAccess = 'owner' | 'edit' | 'view';

export type DashboardShareType = 'user' | 'team' | 'project' | 'organization';
export type DashboardSharePermission = 'view' | 'edit';

export interface DashboardShare {
  readonly type: DashboardShareType;
  readonly targetId: string;
  readonly targetName: string;
  readonly permission: DashboardSharePermission;
}

/** A dashboard. List entries carry `gadgetCount` and no gadgets; the detail carries the gadgets. */
export interface Dashboard {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly layout: DashboardLayout;
  readonly isDefault: boolean;
  readonly ownerId: string;
  readonly ownerName: string;
  readonly organizationId: string;
  readonly access: DashboardAccess;
  readonly canEdit: boolean;
  readonly canManage: boolean;
  readonly isFavorite: boolean;
  readonly shared: boolean;
  readonly gadgetCount: number;
  readonly gadgets: readonly DashboardGadget[];
  /** Only the owner receives the shares; null for everybody else. */
  readonly shares: readonly DashboardShare[] | null;
  readonly updatedAt: string;
}

export type DashboardScope = 'all' | 'mine' | 'shared';

export interface DashboardListQuery {
  readonly scope?: DashboardScope;
  readonly q?: string;
  readonly page?: number;
  readonly pageSize?: number;
}

export interface DashboardPage {
  readonly items: readonly Dashboard[];
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
}

export interface DashboardCreateInput {
  readonly name: string;
  readonly description?: string;
  readonly layout?: DashboardLayout;
}

export interface DashboardPatch {
  readonly name?: string;
  readonly description?: string;
  readonly layout?: DashboardLayout;
  readonly isDefault?: boolean;
}

/** One gadget of a layout replacement. `id` is only set for gadgets the API already stores. */
export interface DashboardGadgetInput {
  readonly id?: string;
  readonly type: GadgetType;
  readonly title: string;
  readonly config: GadgetConfig;
  readonly columnIndex: number;
  readonly position: number;
}

export interface DashboardShareInput {
  readonly type: DashboardShareType;
  readonly targetId: string;
  readonly permission: DashboardSharePermission;
}

// Gadget data (computed by the API with the permissions of the viewer).

export interface GadgetIssueRow {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly status: string;
  readonly statusCategory: string;
  readonly priority: string;
  readonly type: string;
  readonly assignee: string;
  readonly projectKey: string;
  readonly due: string | null;
  readonly updated: string;
}

export interface IssueListGadgetData {
  readonly total: number;
  readonly columns: readonly string[];
  readonly query: string;
  readonly items: readonly GadgetIssueRow[];
}

export interface StatGroup {
  readonly key: string;
  readonly label: string;
  readonly count: number;
}

export interface StatsGadgetData {
  readonly groupBy: string;
  readonly total: number;
  readonly groups: readonly StatGroup[];
  readonly other: number;
}

export interface MatrixCell {
  readonly x: string;
  readonly y: string;
  readonly count: number;
}

export interface MatrixGadgetData {
  readonly xAxis: string;
  readonly yAxis: string;
  readonly total: number;
  readonly xKeys: readonly StatGroup[];
  readonly yKeys: readonly StatGroup[];
  readonly cells: readonly MatrixCell[];
}

export interface DayPoint {
  readonly date: string;
  readonly created: number;
  readonly resolved: number;
}

export interface CreatedVsResolvedGadgetData {
  readonly days: number;
  readonly series: readonly DayPoint[];
  readonly totalCreated: number;
  readonly totalResolved: number;
  readonly netOpenChange: number;
}

export interface GadgetBurndownPoint {
  readonly date: string;
  /** Null for the days after today. */
  readonly remaining: number | null;
  readonly ideal: number;
}

export interface BurndownGadgetData {
  readonly sprint: {
    readonly id: string;
    readonly name: string;
    readonly projectKey: string;
    readonly startDate: string | null;
    readonly endDate: string | null;
  } | null;
  readonly totalPoints: number;
  readonly points: readonly GadgetBurndownPoint[];
}

export interface ActivityGadgetItem {
  readonly id: string;
  readonly issueId: string;
  readonly issueKey: string;
  readonly summary: string;
  readonly actor: string;
  readonly action: string;
  readonly field: string;
  readonly oldValue: string;
  readonly newValue: string;
  readonly occurredAt: string;
}

export interface ActivityGadgetData {
  readonly items: readonly ActivityGadgetItem[];
}

export interface RecentProjectGadgetItem {
  readonly id: string;
  readonly key: string;
  readonly name: string;
  readonly color: string;
  readonly viewedAt: string;
}

export interface RecentProjectsGadgetData {
  readonly items: readonly RecentProjectGadgetItem[];
}

export interface FavoriteGadgetItem {
  readonly type: string;
  readonly id: string;
  readonly name: string;
  readonly link: string;
  readonly starredAt: string;
}

export interface FavoritesGadgetData {
  readonly items: readonly FavoriteGadgetItem[];
}

export interface TextGadgetData {
  readonly markdown: string;
}

export interface UpcomingGadgetItem {
  readonly kind: string;
  readonly title: string;
  readonly startsAt: string;
  readonly endsAt: string;
  readonly allDay: boolean;
  readonly link: string;
  readonly projectKey: string;
}

export interface UpcomingGadgetData {
  readonly from: string;
  readonly to: string;
  readonly items: readonly UpcomingGadgetItem[];
}

/** Computed content of a gadget. The `type` tells which data shape `data` has. */
export type GadgetData =
  | { readonly type: 'assigned_to_me'; readonly data: IssueListGadgetData }
  | { readonly type: 'created_by_me'; readonly data: IssueListGadgetData }
  | { readonly type: 'filter_results'; readonly data: IssueListGadgetData }
  | { readonly type: 'issue_statistics'; readonly data: StatsGadgetData }
  | { readonly type: 'pie'; readonly data: StatsGadgetData }
  | { readonly type: 'two_dimensional'; readonly data: MatrixGadgetData }
  | { readonly type: 'created_vs_resolved'; readonly data: CreatedVsResolvedGadgetData }
  | { readonly type: 'sprint_burndown'; readonly data: BurndownGadgetData }
  | { readonly type: 'activity_stream'; readonly data: ActivityGadgetData }
  | { readonly type: 'recent_projects'; readonly data: RecentProjectsGadgetData }
  | { readonly type: 'favorites'; readonly data: FavoritesGadgetData }
  | { readonly type: 'text'; readonly data: TextGadgetData }
  | { readonly type: 'calendar_upcoming'; readonly data: UpcomingGadgetData };

export interface GadgetDataEnvelope {
  readonly gadgetId: string;
  readonly generatedAt: string;
  readonly content: GadgetData;
}
