import { Routes } from '@angular/router';

/**
 * Automation section of the project settings: the rules list and the builder (new and existing rule). Mounted
 * by the settings area under `automation`.
 */
export const AUTOMATION_ROUTES: Routes = [
  {
    path: '',
    title: 'Automatización',
    loadComponent: () =>
      import('./automation-rules-page/automation-rules-page').then((m) => m.AutomationRulesPage),
  },
  {
    path: 'new',
    title: 'Nueva regla',
    loadComponent: () =>
      import('./rule-editor-page/rule-editor-page').then((m) => m.RuleEditorPage),
  },
  {
    path: ':ruleId',
    title: 'Editar regla',
    loadComponent: () =>
      import('./rule-editor-page/rule-editor-page').then((m) => m.RuleEditorPage),
  },
];
