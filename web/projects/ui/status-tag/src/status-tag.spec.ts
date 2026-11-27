import { TestBed } from '@angular/core/testing';
import { IssueStatus } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { StatusTag } from './status-tag';

function textFor(status: IssueStatus): string {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(StatusTag);
  fixture.componentRef.setInput('status', status);
  fixture.detectChanges();
  return (fixture.nativeElement as HTMLElement).textContent?.trim() ?? '';
}

describe('StatusTag', () => {
  it('renders the open status label', () => {
    expect(textFor('open')).toBe('Abierto');
  });

  it('renders the in_progress status label', () => {
    expect(textFor('in_progress')).toBe('En progreso');
  });

  it('renders the resolved and closed status labels', () => {
    expect(textFor('resolved')).toBe('Resuelto');
    TestBed.resetTestingModule();
    expect(textFor('closed')).toBe('Cerrado');
  });
});

describe('StatusTag with a workflow status', () => {
  it('shows the workflow status name and the color of its category', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(StatusTag);
    fixture.componentRef.setInput('status', 'open');
    fixture.componentRef.setInput('workflowStatus', {
      name: 'En revisión',
      category: 'in_progress',
    });
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent?.trim()).toBe('En revisión');
    expect(el.querySelector('.tone-tag--blue')).not.toBeNull();
  });
});
