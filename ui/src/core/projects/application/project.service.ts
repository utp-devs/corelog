import type { CreateProjectInput, ProjectRepositoryPort } from './ports/project-repository.port'

export class ProjectService {
  private readonly repository: ProjectRepositoryPort

  constructor(repository: ProjectRepositoryPort) {
    this.repository = repository
  }

  list() {
    return this.repository.list()
  }

  create(input: CreateProjectInput) {
    return this.repository.create(input)
  }
}
