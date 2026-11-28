import type { Issue } from './issue.model';

/** Issue returned by the CoreQL search: the list shape plus the labels and the votes. */
export interface SearchedIssue extends Issue {
  readonly labels: readonly string[];
  readonly voteCount: number;
}

export interface IssueSearchWarning {
  readonly code: string;
  readonly message: string;
}

export interface IssueSearchPage {
  readonly items: readonly SearchedIssue[];
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
  /** Query as the server normalized it. */
  readonly query: string;
  readonly warnings: readonly IssueSearchWarning[];
}

export interface IssueSearchParams {
  readonly jql: string;
  /** Sort list (`created DESC, key ASC`); replaces the ORDER BY of the query. */
  readonly order?: string;
  readonly page?: number;
  readonly pageSize?: number;
}

/** One problem found in a query: where it is (in characters), what is wrong and how to fix it. */
export interface CoreqlProblem {
  readonly position: number;
  readonly length: number;
  readonly code: string;
  readonly message: string;
  readonly suggestions: readonly string[];
  readonly expected: readonly string[];
}

export interface CoreqlValidation {
  readonly valid: boolean;
  readonly query: string;
  readonly errors: readonly CoreqlProblem[];
}

/** Completion of the query at the cursor: a field, an operator, a keyword or a function. */
export interface CoreqlCompletion {
  readonly label: string;
  readonly kind: string;
  readonly detail: string;
}

/** Candidate value of a field. `value` is what goes into the query; `label` is what people read. */
export interface CoreqlValueOption {
  readonly value: string;
  readonly label: string;
  readonly detail: string;
}

export interface CoreqlSuggestions {
  readonly completions: readonly CoreqlCompletion[];
  readonly values: readonly CoreqlValueOption[];
}

/** Fields whose values the basic filters offer. */
export type SuggestField =
  | 'project'
  | 'type'
  | 'statusCategory'
  | 'assignee'
  | 'reporter'
  | 'labels'
  | 'component'
  | 'fixVersion'
  | 'priority'
  | 'status';
