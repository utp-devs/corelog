export interface DashboardSprint {
  readonly id: string;
  readonly name: string;
  readonly projectKey: string;
  readonly endDate?: string;
}

export interface DashboardIssue {
  readonly id: string;
  readonly title: string;
  readonly status: string;
  readonly priority: string;
  readonly projectKey: string;
  readonly dueDate?: string;
}

/** Personal summary of the signed-in user. `assignedByStatus` is keyed by the API status value. */
export interface MyDashboard {
  readonly assignedOpen: number;
  readonly assignedByStatus: Readonly<Record<string, number>>;
  readonly overdue: number;
  readonly projectCount: number;
  readonly activeSprints: readonly DashboardSprint[];
  readonly recentIssues: readonly DashboardIssue[];
}
