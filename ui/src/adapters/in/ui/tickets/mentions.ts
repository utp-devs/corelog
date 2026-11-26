import { UserDirectoryHttpRepository } from '@/adapters/out/http/user-directory-http-repository'
import { UserDirectoryService } from '@/core/users/application/user-directory.service'
import type { UserLite } from '@/shared/ui/RichTextEditor'

const directoryService = new UserDirectoryService(new UserDirectoryHttpRepository())

/** Busca compañeros para el selector de menciones (@) del editor de texto enriquecido. */
export async function searchMentionUsers(query: string): Promise<UserLite[]> {
  const users = await directoryService.searchTeammates(query)
  return users.map((user) => ({ id: user.id, name: user.name, email: user.email }))
}
