import { Routes } from '@angular/router';
import { unsavedChangesGuard } from '@corelog/core';

/**
 * Account profile with its sections, mounted by the workspace app under /workspace/profile.
 * The section is the `section` parameter; an unknown or missing section shows the general section.
 * Leaving a route with unsaved edits asks for confirmation.
 */
export const PROFILE_ROUTES: Routes = [
  {
    path: '',
    title: 'Mi perfil',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./profile-page/profile-page').then((m) => m.ProfilePage),
  },
  {
    path: ':section',
    title: 'Mi perfil',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./profile-page/profile-page').then((m) => m.ProfilePage),
  },
];
