import { Injectable, inject } from '@angular/core';
import {
  MentionUser,
  OrganizationStore,
  UserDirectoryApiService,
  toMentionUsers,
} from '@corelog/core';
import { firstValueFrom, map } from 'rxjs';

/** Teammate search for the @ mention menu of the rich text editor. */
@Injectable({ providedIn: 'root' })
export class MentionSearch {
  private readonly directory = inject(UserDirectoryApiService);
  private readonly organizations = inject(OrganizationStore);

  /** Searches teammates of the active organization. Without an organization there is nothing to find. */
  readonly search = (query: string): Promise<MentionUser[]> => {
    const organizationId = this.organizations.currentOrgId();
    if (!organizationId) return Promise.resolve([]);
    return firstValueFrom(
      this.directory
        .searchTeammates(organizationId, query)
        .pipe(map((users) => toMentionUsers(users))),
    );
  };
}
