export type FilterAccess = 'owner' | 'edit' | 'view';
export type FilterShareType = 'user' | 'team' | 'project' | 'organization';
export type FilterSharePermission = 'view' | 'edit';
export type FilterScope = 'all' | 'mine' | 'shared' | 'favorite';
export type SubscriptionFrequency = 'daily' | 'weekly';

export interface FilterShare {
  readonly type: FilterShareType;
  readonly targetId: string;
  readonly targetName: string;
  readonly permission: FilterSharePermission;
}

export interface FilterSubscription {
  readonly frequency: SubscriptionFrequency;
  /** Hour of the day, 0 to 23. */
  readonly hour: number;
  /** 0 (Sunday) to 6; only for the weekly frequency. */
  readonly weekday: number;
  readonly timezone: string;
}

/** A saved CoreQL filter. */
export interface SavedQuery {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly query: string;
  readonly columns: readonly string[];
  readonly sort: string;
  readonly ownerId: string;
  readonly ownerName: string;
  readonly organizationId: string;
  readonly access: FilterAccess;
  readonly isFavorite: boolean;
  readonly isSubscribed: boolean;
  readonly subscription: FilterSubscription | null;
  readonly shares: readonly FilterShare[];
  readonly updatedAt: string;
}

export interface SaveQueryPayload {
  readonly name: string;
  readonly description?: string;
  readonly query: string;
  readonly columns?: readonly string[];
  readonly sort?: string;
  readonly organizationId?: string;
}

export interface FilterShareInput {
  readonly type: FilterShareType;
  readonly targetId: string;
  readonly permission: FilterSharePermission;
}
