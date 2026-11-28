import { Routes } from '@angular/router';

/** Import section of the project settings, mounted under `import`. */
export const IMPORT_ROUTES: Routes = [
  {
    path: '',
    title: 'Importar incidencias',
    loadComponent: () => import('./import-page/import-page').then((m) => m.ImportPage),
  },
];

/** Export section of the project settings, mounted under `export`. */
export const EXPORT_ROUTES: Routes = [
  {
    path: '',
    title: 'Exportar',
    loadComponent: () => import('./export-page/export-page').then((m) => m.ExportPage),
  },
];
