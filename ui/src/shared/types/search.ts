/** Search vocabulary shared by the design system and the domain. core/search re-exports these names. */
export type SearchKind = 'project' | 'issue'

export interface SearchHit {
  kind: SearchKind
  id: string
  title: string
  subtitle: string
  link: string
}
