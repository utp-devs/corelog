import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { AuthStore, LOCATION_ASSIGN, OrganizationStore } from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { Button } from 'primeng/button';

import { ChooseStep } from './choose-step';
import { InviteStep } from './invite-step';
import { OrgFormStep } from './org-form-step';

interface StepInfo {
  readonly title: string;
  readonly optional: boolean;
}

const STEPS: readonly StepInfo[] = [
  { title: 'Elegir', optional: false },
  { title: 'Organización', optional: false },
  { title: 'Equipo', optional: true },
  { title: 'Finalizar', optional: false },
];

/**
 * Onboarding wizard, authenticated but outside the app shell. Step 1 chooses between creating an
 * organization and joining one; `?new=1` starts at step 2 (used by the organization switcher).
 * The header always offers a way out: sign out, or cancel when the user already has a workspace.
 */
@Component({
  selector: 'app-onboarding-page',
  imports: [Button, PageContainer, ChooseStep, OrgFormStep, InviteStep],
  templateUrl: './onboarding-page.html',
  styleUrl: './onboarding-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OnboardingPage {
  private readonly router = inject(Router);
  /** Full-page move to the workspace: the workspace is a separate application. */
  private readonly assign = inject(LOCATION_ASSIGN);
  private readonly auth = inject(AuthStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly route = inject(ActivatedRoute);

  protected readonly steps = STEPS;
  protected readonly org = this.orgs.currentOrg;
  /** Opened from the organization switcher: the user already has a workspace to go back to. */
  protected readonly canCancel = this.route.snapshot.queryParamMap.get('new') !== null;
  protected readonly step = signal<number>(this.canCancel ? 1 : 0);
  protected readonly orgName = computed(() => this.org()?.name ?? null);
  protected readonly stepLabel = computed(
    () => `Paso ${this.step() + 1} de ${STEPS.length}: ${STEPS[this.step()].title}`,
  );

  protected stateOf(index: number): 'done' | 'current' | 'upcoming' {
    const current = this.step();
    if (index < current) return 'done';
    return index === current ? 'current' : 'upcoming';
  }

  protected finish(): void {
    this.assign('/workspace/projects');
  }

  protected cancel(): void {
    this.assign('/workspace/');
  }

  protected signOut(): void {
    this.auth.logout();
    void this.router.navigateByUrl('/login', { replaceUrl: true });
  }
}
