import { Routes } from '@angular/router';

/** Organization webhooks, mounted by the admin app under /organization/webhooks. */
export const ORG_WEBHOOKS_ROUTES: Routes = [
  {
    path: '',
    title: 'Webhooks',
    loadComponent: () => import('./webhooks/webhooks-page').then((m) => m.WebhooksPage),
  },
];
