import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { publicRequest } from '../../config/http-context';
import type { Country } from '../../models/country.model';

interface CountryDto {
  readonly code: string;
  readonly name: string;
  readonly dial_code: string;
  readonly example?: string;
  readonly areas?: readonly string[];
}

/** Phone catalog: countries with their international prefix. Public. */
@Injectable({ providedIn: 'root' })
export class CountryApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(): Observable<Country[]> {
    return this.http
      .get<{ data: CountryDto[] }>(`${this.baseUrl}/meta/countries`, { context: publicRequest() })
      .pipe(
        map((r) =>
          r.data.map((d) => ({
            code: d.code,
            name: d.name,
            dialCode: d.dial_code,
            example: d.example ?? null,
            areas: d.areas ?? [],
          })),
        ),
      );
  }
}
