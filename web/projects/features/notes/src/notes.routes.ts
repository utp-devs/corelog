import { Routes } from '@angular/router';

/**
 * Personal notes of the signed-in user, mounted by the workspace app under /workspace/notes.
 * Paths are relative to the mount point. `?note=<id>` opens that note in the editor.
 */
export const NOTES_ROUTES: Routes = [
  {
    path: '',
    title: 'Mis apuntes',
    loadComponent: () => import('./notes-page/notes-page').then((m) => m.NotesPage),
  },
];
