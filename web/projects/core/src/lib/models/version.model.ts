export type VersionStatus = 'unreleased' | 'released' | 'archived';

export interface VersionProgress {
  readonly total: number;
  readonly done: number;
  readonly inProgress: number;
  readonly todo: number;
  readonly pointsTotal: number;
  readonly pointsDone: number;
}

export interface ReleaseVersion {
  readonly id: string;
  readonly projectId: string;
  readonly name: string;
  readonly description: string;
  readonly startDate: string | null;
  readonly releaseDate: string | null;
  readonly status: VersionStatus;
  readonly releasedAt: string | null;
  readonly progress: VersionProgress;
}

export interface VersionPayload {
  readonly name: string;
  readonly description?: string;
  readonly startDate?: string | null;
  readonly releaseDate?: string | null;
}

/** Where the unfinished issues go when a version is released: `new` creates the next version. */
export type MoveUnresolvedTo = string | null;

export interface ReleaseResult {
  readonly version: ReleaseVersion;
  readonly movedIssues: number;
  readonly unresolvedIssues: number;
  readonly nextVersion: ReleaseVersion | null;
}

export interface ReleaseNotesIssue {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly done: boolean;
}

export interface ReleaseNotes {
  readonly versionName: string;
  readonly status: VersionStatus;
  readonly releaseDate: string | null;
  readonly issueCount: number;
  readonly markdown: string;
  readonly groups: ReadonlyArray<{ title: string; issues: readonly ReleaseNotesIssue[] }>;
}
