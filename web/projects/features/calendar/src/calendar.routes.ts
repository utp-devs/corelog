import { Routes } from '@angular/router';

/** Combined calendar of the signed-in user, mounted by the workspace app under /workspace/calendar. */
export const CALENDAR_ROUTES: Routes = [
  {
    path: '',
    title: 'Calendario',
    loadComponent: () => import('./calendar-page/calendar-page').then((m) => m.CalendarPage),
  },
];
