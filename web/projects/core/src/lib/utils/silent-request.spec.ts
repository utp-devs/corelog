import { SKIP_ERROR_NOTIFICATION, SKIP_LOADING } from '../config/http-context';
import { backgroundRequest, silentRequest } from './silent-request';

describe('request contexts', () => {
  it('silent requests skip the error toast but keep the loading bar', () => {
    const context = silentRequest();
    expect(context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    expect(context.get(SKIP_LOADING)).toBe(false);
  });

  it('background requests skip both the error toast and the loading bar', () => {
    const context = backgroundRequest();
    expect(context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    expect(context.get(SKIP_LOADING)).toBe(true);
  });

  it('creates a new context on every call', () => {
    expect(silentRequest()).not.toBe(silentRequest());
  });
});
