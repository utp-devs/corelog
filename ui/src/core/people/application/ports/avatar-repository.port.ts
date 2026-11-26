export interface AvatarRepositoryPort {
  /** Authenticated PNG of any user. Rejects with a 404 AppError when the user has no avatar. */
  getAvatar(userId: string): Promise<Blob>
  /** Replaces the caller's avatar and resolves with the relative avatar URL. */
  uploadMyAvatar(file: File): Promise<string>
  deleteMyAvatar(): Promise<void>
}
