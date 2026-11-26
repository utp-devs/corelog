import { Avatar, type AvatarSize } from '@/shared/ui/Avatar'

import { useAvatarUrl } from './useAvatarUrl'

type UserAvatarProps = {
  id: string
  name: string
  /** Whether the user has a photo (API has_avatar). When false no image is requested and initials are shown. */
  hasAvatar?: boolean
  /** Optional cache version (for example the avatar key of the signed-in user) so a new photo is fetched again. */
  avatarKey?: string | null
  size?: AvatarSize
  'data-testid'?: string
}

/** Avatar with the user's photo (fetched with auth, only when hasAvatar is true) or initials. */
export function UserAvatar({ id, name, hasAvatar, avatarKey, size = 32, 'data-testid': testId }: UserAvatarProps) {
  const withPhoto = hasAvatar ?? Boolean(avatarKey)
  const src = useAvatarUrl(id, withPhoto, avatarKey ?? '')
  return <Avatar id={id} name={name} src={src} size={size} data-testid={testId} />
}
