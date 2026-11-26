/** Logo fetch used by the logo tiles. Implemented by the HTTP repository. */
export interface OrganizationLogoPort {
  /** Logo of an organization, fetched with the session. Resolves with null when there is no logo or the request fails. */
  fetchLogoBlob(organizationId: string): Promise<Blob | null>
}
