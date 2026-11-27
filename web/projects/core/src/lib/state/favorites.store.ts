import { Injectable, computed, inject } from '@angular/core';
import { Observable, catchError, throwError } from 'rxjs';

import { FavoriteItem, RecentProjectItem } from '../models';
import { FavoritesApiService } from '../services/api/favorites-api.service';
import { storeError } from '../utils/store-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** What is needed to show a project as a favorite right after starring it. */
export interface StarrableProject {
  readonly id: string;
  readonly name: string;
  readonly key: string;
  readonly color?: string;
}

/**
 * Starred projects and recently visited projects of the signed-in user. `star` and `unstar` update
 * the list at once (optimistically) and undo the change when the request fails.
 */
@Injectable({ providedIn: 'root' })
export class FavoritesStore {
  private readonly api = inject(FavoritesApiService);

  private readonly favoritesRead = new AsyncState<FavoriteItem[]>();
  private readonly recentRead = new AsyncState<RecentProjectItem[]>();

  readonly favorites = computed(() => this.favoritesRead.value() ?? []);
  readonly favoritesLoading = this.favoritesRead.loading;
  readonly favoritesError = this.favoritesRead.error;
  readonly recent = computed(() => this.recentRead.value() ?? []);
  readonly recentLoading = this.recentRead.loading;
  readonly recentError = this.recentRead.error;

  private readonly starredIds = computed(() => new Set(this.favorites().map((item) => item.id)));

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Reads the signal; call it inside a computed or a template to follow the star of a project. */
  isStarred(projectId: string): boolean {
    return this.starredIds().has(projectId);
  }

  fetchFavorites(): void {
    this.favoritesRead.load(this.api.list('project'));
  }

  fetchRecent(limit?: number): void {
    this.recentRead.load(this.api.recent('project', limit));
  }

  star(project: StarrableProject): Observable<void> {
    const previous = this.favoritesRead.value();
    const item: FavoriteItem = {
      type: 'project',
      id: project.id,
      name: project.name,
      key: project.key,
      color: project.color ?? '',
      createdAt: new Date().toISOString(),
    };
    this.favoritesRead.update((items) => [
      item,
      ...(items ?? []).filter((existing) => existing.id !== project.id),
    ]);
    return this.api.star('project', project.id).pipe(
      catchError((error: unknown) => {
        this.favoritesRead.set(previous);
        return this.fail(error, 'No se pudo marcar el proyecto como favorito');
      }),
    );
  }

  unstar(projectId: string): Observable<void> {
    const previous = this.favoritesRead.value();
    this.favoritesRead.update((items) => items?.filter((item) => item.id !== projectId));
    return this.api.unstar('project', projectId).pipe(
      catchError((error: unknown) => {
        this.favoritesRead.set(previous);
        return this.fail(error, 'No se pudo quitar el proyecto de favoritos');
      }),
    );
  }

  reset(): void {
    this.favoritesRead.reset();
    this.recentRead.reset();
  }

  private fail(error: unknown, fallbackMessage: string): Observable<never> {
    return throwError(() => storeError(error, fallbackMessage));
  }
}
