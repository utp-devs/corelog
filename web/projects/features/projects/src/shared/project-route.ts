import { Signal, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { map } from 'rxjs';

/** Project id taken from the parent route (the project workspace), for the section routes. */
export function injectProjectId(): Signal<string> {
  const route = inject(ActivatedRoute);
  const source = route.parent ?? route;
  return toSignal(source.paramMap.pipe(map((params) => params.get('id') ?? '')), {
    requireSync: true,
  });
}
