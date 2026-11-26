import { ErrorPage } from '@/shared/ui/ErrorPage'

/** Unknown route (S-06): the shared 404 page with a link back home. */
export function NotFoundPage() {
  return <ErrorPage variant={404} />
}
