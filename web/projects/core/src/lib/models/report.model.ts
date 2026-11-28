/** Report kinds the API serves (the path segment of `/projects/{project}/reports/{kind}`). */
export const REPORT_KINDS = [
  'burndown',
  'burnup',
  'velocity',
  'cumulative-flow',
  'control',
  'created-vs-resolved',
  'resolution-time',
  'time-tracking',
  'workload',
  'version',
  'sprint-report',
] as const;
export type ReportKind = (typeof REPORT_KINDS)[number];

/** Type of a table column: drives the formatting of its cells. */
export type ReportColumnType = 'text' | 'number' | 'percent' | 'date';

/** One entry of the report catalog: what it is and the query parameters it accepts. */
export interface ReportDescriptor {
  readonly kind: string;
  readonly title: string;
  readonly description: string;
  readonly params: readonly string[];
}

export interface ReportColumn {
  readonly key: string;
  readonly label: string;
  readonly type: ReportColumnType;
}

export interface ReportPoint {
  /** Date (`YYYY-MM-DD`), period or label. */
  readonly x: string;
  readonly y: number;
}

export interface ReportSeries {
  readonly name: string;
  readonly points: readonly ReportPoint[];
}

export type ReportCell = string | number | boolean | null;

export interface ReportTable {
  readonly columns: readonly ReportColumn[];
  /** One value per column, in the order of `columns`. */
  readonly rows: readonly (readonly ReportCell[])[];
}

/** Uniform result of every report: series for the chart, a table and a summary of totals. */
export interface Report {
  readonly kind: string;
  readonly title: string;
  readonly generatedAt: string;
  readonly params: Readonly<Record<string, unknown>>;
  readonly series: readonly ReportSeries[];
  readonly table: ReportTable;
  readonly summary: Readonly<Record<string, unknown>>;
}

/** Query parameters of a report. Empty values are not sent. */
export interface ReportQuery {
  readonly from?: string;
  readonly to?: string;
  readonly groupBy?: string;
  readonly filter?: string;
  readonly sprint?: string;
  readonly version?: string;
  readonly interval?: string;
  readonly limit?: number;
}
