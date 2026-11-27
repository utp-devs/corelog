import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Button } from 'primeng/button';
import { OrganizationStore } from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';

import { ChooseStep } from './choose-step';
import { InviteStep } from './invite-step';
import { OrgFormStep } from './org-form-step';

const STEP_TITLES = ['Elegir', 'Organización', 'Equipo', 'Finalizar'] as const;

/**
 * Onboarding wizard, authenticated but outside the app shell. Step 1 chooses between creating an
 * organization and joining one; `?new=1` starts at step 2 (used by the organization switcher).
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
  private readonly orgs = inject(OrganizationStore);
  private readonly route = inject(ActivatedRoute);

  protected readonly stepTitles = STEP_TITLES;
  protected readonly org = this.orgs.currentOrg;
  protected readonly step = signal<number>(this.route.snapshot.queryParamMap.get('new') ? 1 : 0);
  protected readonly orgName = computed(() => this.org()?.name ?? null);

  protected stateOf(index: number): 'done' | 'current' | 'upcoming' {
    const current = this.step();
    if (index < current) return 'done';
    return index === current ? 'current' : 'upcoming';
  }

  protected finish(): void {
    void this.router.navigateByUrl('/projects', { replaceUrl: true });
  }
}
