import { Directive } from '@angular/core';

/**
 * Marks the organization switcher inside `<app-shell>`. The top bar shows it on desktop and tablet;
 * on phones the shell moves it into the navigation drawer.
 */
@Directive({ selector: '[appShellOrgSwitcher]' })
export class AppShellOrgSwitcher {}

/** Marks a custom create button area inside `<app-shell>`. It replaces the default Crear button. */
@Directive({ selector: '[appShellCreate]' })
export class AppShellCreate {}
