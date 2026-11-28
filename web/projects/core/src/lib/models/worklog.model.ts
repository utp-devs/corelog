export type RemainingMode = 'auto' | 'set' | 'reduce' | 'leave';

export interface Worklog {
  readonly id: string;
  readonly issueId: string;
  readonly issueKey: string;
  readonly authorId: string;
  readonly authorName: string;
  readonly startedAt: string;
  readonly timeSpentSeconds: number;
  readonly timeSpent: string;
  readonly description: string;
  readonly remainingMode: RemainingMode;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface WorklogTotals {
  readonly originalEstimateSeconds: number | null;
  readonly remainingEstimateSeconds: number | null;
  readonly timeSpentSeconds: number;
  readonly originalEstimate: string | null;
  readonly remainingEstimate: string | null;
  readonly timeSpent: string;
}

export interface WorklogList {
  readonly worklogs: readonly Worklog[];
  readonly totals: WorklogTotals;
}

export interface LogWorkInput {
  readonly timeSpent: string;
  readonly startedAt?: string;
  readonly description?: string;
  readonly remaining?: { readonly mode: RemainingMode; readonly value?: string };
}

export interface UpdateWorklogInput {
  readonly timeSpent?: string;
  readonly startedAt?: string;
  readonly description?: string;
}

export interface TimesheetEntry {
  readonly id: string;
  readonly issueId: string;
  readonly issueKey: string;
  readonly issueTitle: string;
  readonly startedAt: string;
  readonly timeSpentSeconds: number;
  readonly timeSpent: string;
  readonly description: string;
}

export interface TimesheetDay {
  readonly date: string;
  readonly totalSeconds: number;
  readonly total: string;
  readonly capacitySeconds: number;
  readonly entries: readonly TimesheetEntry[];
}

export interface Timesheet {
  readonly from: string;
  readonly to: string;
  readonly totalSeconds: number;
  readonly total: string;
  readonly days: readonly TimesheetDay[];
}
