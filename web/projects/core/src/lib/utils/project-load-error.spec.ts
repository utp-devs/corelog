import { ApiError } from '../models';
import { projectAccessVariantOf } from './project-load-error';

const failure = (status: number): ApiError => ({
  status,
  code: `http.${status}`,
  message: 'mensaje',
  details: [],
});

describe('projectAccessVariantOf', () => {
  it('maps forbidden and not found to their full-page variants', () => {
    expect(projectAccessVariantOf(failure(403))).toBe(403);
    expect(projectAccessVariantOf(failure(404))).toBe(404);
  });

  it('returns null for other failures and for no failure', () => {
    expect(projectAccessVariantOf(failure(500))).toBeNull();
    expect(projectAccessVariantOf(null)).toBeNull();
    expect(projectAccessVariantOf(undefined)).toBeNull();
  });
});
