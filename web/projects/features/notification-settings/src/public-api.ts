import { Routes } from '@angular/router';

export { ProjectNotificationsPage } from './pages/project-notifications-page';
export { OrgNotificationsPage } from './pages/org-notifications-page';
export { SchemeEditor } from './scheme-editor/scheme-editor';

export const ORG_NOTIFICATION_ROUTES: Routes = [
  {
    path: 'notifications',
    title: 'Notificaciones de la organización',
    loadComponent: () =>
      import('./pages/org-notifications-page').then((m) => m.OrgNotificationsPage),
  },
];
