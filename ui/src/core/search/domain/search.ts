export type SearchKind = 'project' | 'issue'

export interface SearchHit {
  kind: SearchKind
  id: string
  title: string
  subtitle: string
  link: string
}
