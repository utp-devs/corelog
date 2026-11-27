import {
  ApplicationRef,
  DOCUMENT,
  EnvironmentInjector,
  OutputRefSubscription,
  Provider,
  createComponent,
  inject,
} from '@angular/core';
import { UNSAVED_CHANGES_PROMPT, UnsavedChangesPromptCopy } from '@corelog/core';

import { ConfirmModal } from './confirm-modal';

/**
 * Shows the shared confirmation dialog without a host in the page: the dialog is created on demand,
 * attached to the application and destroyed after the user answers.
 */
function askToLeave(
  environment: EnvironmentInjector,
  appRef: ApplicationRef,
  document: Document,
  copy: UnsavedChangesPromptCopy,
): Promise<boolean> {
  return new Promise<boolean>((resolve) => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const ref = createComponent(ConfirmModal, {
      environmentInjector: environment,
      hostElement: host,
    });
    const subscriptions: OutputRefSubscription[] = [];
    let settled = false;

    const finish = (leave: boolean) => {
      if (settled) return;
      settled = true;
      subscriptions.forEach((subscription) => subscription.unsubscribe());
      appRef.detachView(ref.hostView);
      ref.destroy();
      host.remove();
      resolve(leave);
    };

    subscriptions.push(
      ref.instance.confirmed.subscribe(() => finish(true)),
      ref.instance.cancelled.subscribe(() => finish(false)),
    );
    ref.setInput('open', true);
    ref.setInput('title', copy.title);
    ref.setInput('description', copy.description);
    ref.setInput('confirmText', copy.confirmText);
    ref.setInput('cancelText', copy.cancelText);
    appRef.attachView(ref.hostView);
    appRef.tick();
  });
}

/** Replaces the default browser prompt with the shared confirmation dialog. */
export function provideUnsavedChangesDialog(): Provider {
  return {
    provide: UNSAVED_CHANGES_PROMPT,
    useFactory: () => {
      const environment = inject(EnvironmentInjector);
      const appRef = inject(ApplicationRef);
      const document = inject(DOCUMENT);
      return (copy: UnsavedChangesPromptCopy) => askToLeave(environment, appRef, document, copy);
    },
  };
}
