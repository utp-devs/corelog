import type { APIRequestContext } from '@playwright/test';

/** Id of an organization the caller belongs to, by slug (org list of the caller). Throws when absent. */
export async function orgIdBySlug(client: APIRequestContext, slug: string): Promise<string> {
  const response = await client.get('organizations');
  const listed = ((await response.json()) as { data: Array<{ id: string; slug: string }> }).data;
  const found = listed.find((org) => org.slug === slug);
  if (!found)
    throw new Error(`organization "${slug}" is not visible to this account (run the e2e seed)`);
  return found.id;
}

/** Accepts an invitation through the API as the account behind `userToken` (its email must match). */
export async function acceptInvitationApi(
  apiURL: string,
  inviteToken: string,
  userToken: string,
): Promise<void> {
  const response = await fetch(`${apiURL}/invitations/${inviteToken}/accept`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${userToken}` },
  });
  if (response.status !== 201)
    throw new Error(`accepting the invitation returned ${response.status}`);
}
