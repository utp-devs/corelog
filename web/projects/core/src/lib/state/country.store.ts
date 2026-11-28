import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type { Country } from '../models/country.model';
import { CountryApiService } from '../services/api/country-api.service';
import { browserCountry } from '../utils/phone-format';

/** Country used when the browser language gives no region the catalog knows. */
export const DEFAULT_COUNTRY_CODE = 'PE';

/** Phone catalog, loaded once and kept for the lifetime of the page. */
@Injectable({ providedIn: 'root' })
export class CountryStore {
  private readonly api = inject(CountryApiService);
  private pending: Promise<void> | null = null;

  readonly countries = signal<readonly Country[]>([]);
  readonly status = signal<'idle' | 'loading' | 'ready' | 'error'>('idle');
  private readonly byCode = computed(() => new Map(this.countries().map((c) => [c.code, c])));

  find(code: string | null | undefined): Country | undefined {
    return code ? this.byCode().get(code.toUpperCase()) : undefined;
  }

  /** Country to preselect: the browser region when the catalog has it, else Peru, else the first. */
  defaultCountry(languages: readonly string[] = []): Country | undefined {
    return (
      browserCountry(this.countries(), languages) ??
      this.find(DEFAULT_COUNTRY_CODE) ??
      this.countries()[0]
    );
  }

  /** Loads the catalog once; later calls share the same request. */
  load(): Promise<void> {
    if (this.status() === 'ready') return Promise.resolve();
    this.pending ??= this.fetch();
    return this.pending;
  }

  private async fetch(): Promise<void> {
    this.status.set('loading');
    try {
      this.countries.set(await firstValueFrom(this.api.list()));
      this.status.set('ready');
    } catch {
      this.status.set('error');
    } finally {
      this.pending = null;
    }
  }
}
