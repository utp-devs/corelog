import { Avatar, type AvatarSize } from '@/shared/ui/Avatar'

import { useAvatarUrl } from './useAvatarUrl'

type UserAvatarProps = {
  id: string
  name: string
  /** Avatar key from the user; null or undefined means the user has no photo. */
  avatarKey?: string | null
  size?: AvatarSize
  'data-testid'?: string
}

/** Avatar with the user's photo (fetched with auth) or initials. */
export function UserAvatar({ id, name, avatarKey, size = 32, 'data-testid': testId }: UserAvatarProps) {
  const hasAvatar = Boolean(avatarKey)
  const src = useAvatarUrl(id, hasAvatar, avatarKey ?? '')
  return <Avatar id={id} name={name} src={src} size={size} data-testid={testId} />
}
