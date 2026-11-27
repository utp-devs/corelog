import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import {
  AuthStore,
  HOME_URL,
  InvitationPreview,
  LOCATION_ASSIGN,
  NotificationService,
  OrganizationStore,
  ROLE_LABELS,
  asApiError,
  authPathWithRedirect,
  invitationErrorMessage,
} from '@corelog/core';
import { OrgLogo } from '@corelog/ui/org-logo';
import { Button } from 'primeng/button';
import { ProgressSpinner } from 'primeng/progressspinner';
import { BehaviorSubject, Observable, catchError, combineLatest, map, of, switchMap } from 'rxjs';

import { AuthAlert } from '../shared/auth-alert';
import { AuthLayout } from '../shared/auth-layout';

const LOAD_FAILED_MESSAGE = 'No se pudo cargar la invitación.';
const INVALID_LINK_MESSAGE = 'Este enlace no es válido.';
const UNUSABLE_MESSAGES: Readonly<Record<string, string>> = {
  accepted: 'Esta invitación ya se ha utilizado. Si ya eres miembro, entra desde tu organización.',
  revoked: 'Esta invitación fue revocada. Pide a un administrador que te envíe una nueva.',
  expired: 'La invitación ha caducado. Pide a un administrador un enlace nuevo.',
};

type PreviewState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'invalid' }
  | { readonly kind: 'failed'; readonly message: string }
  | { readonly kind: 'ready'; readonly preview: InvitationPreview };

/** Public invitation page: previews the invitation and lets the signed-in invitee accept it. */
@Component({
  selector: 'app-invite-accept-page',
  imports: [AuthLayout, AuthAlert, OrgLogo, Button, ProgressSpinner],
  templateUrl: './invite-accept-page.html',
  styleUrl: './invite-accept-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InviteAcceptPage {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly auth = inject(AuthStore);
  private readonly org = inject(OrganizationStore);
  private readonly notifications = inject(NotificationService);
  private readonly home = inject(HOME_URL);
  private readonly assign = inject(LOCATION_ASSIGN);

  protected readonly sessionChecked = this.auth.sessionChecked;
  protected readonly user = this.auth.user;
  protected readonly roleLabels = ROLE_LABELS;

  protected readonly token = signal('');
  protected readonly state = signal<PreviewState>({ kind: 'loading' });
  protected readonly accepting = signal(false);
  protected readonly acceptError = signal<string | null>(null);
  protected readonly alreadyMember = signal(false);
  private readonly reload = new BehaviorSubject(0);

  /** Preview of a loaded invitation, or null while loading or when it could not be shown. */
  protected readonly ready = computed<InvitationPreview | null>(() => {
    const current = this.state();
    return current.kind === 'ready' ? current.preview : null;
  });

  /** Copy for the state where the invitation could not be shown. */
  protected readonly failureMessage = computed(() => {
    const current = this.state();
    if (current.kind === 'failed') return current.message;
    return current.kind === 'invalid' ? INVALID_LINK_MESSAGE : LOAD_FAILED_MESSAGE;
  });

  protected readonly heading = computed(() => {
    const preview = this.ready();
    if (preview) return `Invitación a ${preview.organizationName}`;
    return this.state().kind === 'loading' ? 'Invitación' : 'Invitación no disponible';
  });

  protected readonly subheading = computed(() => {
    const preview = this.ready();
    if (preview) {
      return preview.status === 'pending'
        ? `Te invitan a unirte como ${ROLE_LABELS[preview.role]}.`
        : 'Esta invitación ya no se puede aceptar.';
    }
    return this.state().kind === 'loading'
      ? 'Comprobando la invitación...'
      : 'No pudimos abrir esta invitación.';
  });

  protected readonly returnPath = computed(() => `/invite/${encodeURIComponent(this.token())}`);

  /** Signed-in account matches the invited email (case-insensitive). */
  protected readonly sameAccount = computed(() => {
    const user = this.user();
    const state = this.state();
    if (!user || state.kind !== 'ready') return false;
    return user.email.toLowerCase() === state.preview.email.toLowerCase();
  });

  constructor() {
    combineLatest([this.route.paramMap, this.reload])
      .pipe(
        map(([params]) => params.get('token') ?? ''),
        switchMap((token) => {
          this.token.set(token);
          this.state.set({ kind: 'loading' });
          this.acceptError.set(null);
          this.alreadyMember.set(false);
          return this.loadPreview(token);
        }),
        takeUntilDestroyed(),
      )
      .subscribe((state) => this.state.set(state));
  }

  protected unusableMessage(status: string): string {
    return UNUSABLE_MESSAGES[status] ?? 'Esta invitación no está disponible.';
  }

  protected accept(preview: InvitationPreview): void {
    this.accepting.set(true);
    this.acceptError.set(null);
    this.org.acceptInvitation(this.token()).subscribe({
      next: (membership) => {
        this.org.fetchOrgs();
        this.org.setCurrentOrg(membership.organizationId);
        this.notifications.success(`Te has unido a ${preview.organizationName}`);
        this.assign(this.home);
      },
      error: (error: unknown) => {
        if (asApiError(error).code === 'org.already_member') {
          this.alreadyMember.set(true);
          this.accepting.set(false);
          return;
        }
        this.acceptError.set(invitationErrorMessage(error, preview.email));
        this.accepting.set(false);
      },
    });
  }

  protected switchAccount(): void {
    this.auth.logout();
    void this.router.navigateByUrl(authPathWithRedirect('/login', this.returnPath()), {
      replaceUrl: true,
    });
  }

  protected retry(): void {
    this.reload.next(this.reload.value + 1);
  }

  protected goHome(): void {
    void this.router.navigateByUrl('/');
  }

  protected openWorkspace(): void {
    this.assign(this.home);
  }

  protected goToLogin(): void {
    void this.router.navigateByUrl(authPathWithRedirect('/login', this.returnPath()));
  }

  protected goToRegister(): void {
    void this.router.navigateByUrl(authPathWithRedirect('/register', this.returnPath()));
  }

  private loadPreview(token: string): Observable<PreviewState> {
    return this.org.previewInvitation(token).pipe(
      map((preview): PreviewState => ({ kind: 'ready', preview })),
      catchError((error: unknown) => {
        const apiError = asApiError(error);
        const state: PreviewState =
          apiError.status === 404
            ? { kind: 'invalid' }
            : { kind: 'failed', message: apiError.message || LOAD_FAILED_MESSAGE };
        return of(state);
      }),
    );
  }
}
