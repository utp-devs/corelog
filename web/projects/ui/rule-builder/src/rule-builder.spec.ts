import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type { AutomationRuleDraft } from '@corelog/core';

import { RuleBuilder } from './rule-builder';
import { emptyRule } from './rule-logic';

@Component({
  imports: [RuleBuilder],
  template: `<app-rule-builder
    [rule]="rule()"
    (ruleChange)="rule.set($event)"
    [statuses]="['Hecho']"
  />`,
})
class Host {
  readonly rule = signal<AutomationRuleDraft>(emptyRule());
}

function setup() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const click = (testId: string, index = 0): void => {
    el.querySelectorAll<HTMLElement>(`[data-testid="${testId}"]`)[index]?.click();
    fixture.detectChanges();
  };
  return { fixture, el, host: fixture.componentInstance, click };
}

describe('RuleBuilder', () => {
  it('reads the rule back as a sentence while it is edited', () => {
    const { fixture, el, host } = setup();
    expect(el.querySelector('[data-testid="rule-summary"]')?.textContent).toContain(
      'Cuando se cree una incidencia',
    );
    const area = el.querySelector<HTMLTextAreaElement>('[data-testid="rule-action-text"]')!;
    area.value = 'Hola';
    area.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(host.rule().actions[0].text).toBe('Hola');
    expect(el.querySelector('[data-testid="rule-summary"]')?.textContent).toContain('«Hola»');
  });

  it('adds, reorders and removes actions', () => {
    const { host, click } = setup();
    click('rule-add-action');
    expect(host.rule().actions).toHaveLength(2);
    const second = host.rule().actions[1];
    click('rule-action-up', 1);
    expect(host.rule().actions[0]).toEqual(second);
    click('rule-action-remove', 0);
    expect(host.rule().actions).toHaveLength(1);
  });

  it('inserts a smart value at the caret of the focused text', () => {
    const { fixture, el, host, click } = setup();
    const area = el.querySelector<HTMLTextAreaElement>('[data-testid="rule-action-text"]')!;
    area.value = 'Hola ';
    area.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    area.focus();
    area.setSelectionRange(5, 5);
    area.dispatchEvent(new Event('focus'));
    click('rule-smart-issue.key');
    expect(host.rule().actions[0].text).toBe('Hola {{issue.key}}');
  });

  it('shows the parameters of the chosen trigger', () => {
    const { fixture, el } = setup();
    const select = el.querySelector<HTMLSelectElement>('[data-testid="rule-trigger"]')!;
    select.value = 'issue_transitioned';
    select.dispatchEvent(new Event('change'));
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="rule-trigger-to"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="rule-trigger-every"]')).toBeNull();
  });
});
