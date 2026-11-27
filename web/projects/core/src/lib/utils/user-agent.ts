const MAX_RAW_LENGTH = 60;

/**
 * Reduces a user agent string to "Navegador · Sistema" for display. Unknown clients are shown as they are,
 * trimmed, so that useful information is not hidden (for example curl).
 */
export function describeUserAgent(userAgent: string): string {
  const ua = userAgent.trim();
  if (!ua) return 'Navegador sin identificar';

  const browser = /Edg\//.test(ua)
    ? 'Edge'
    : /OPR\/|Opera/.test(ua)
      ? 'Opera'
      : /Firefox\//.test(ua)
        ? 'Firefox'
        : /Chrome\/|HeadlessChrome\//.test(ua)
          ? 'Chrome'
          : /Safari\//.test(ua)
            ? 'Safari'
            : null;
  const system = /Windows/.test(ua)
    ? 'Windows'
    : /Android/.test(ua)
      ? 'Android'
      : /iPhone|iPad/.test(ua)
        ? 'iOS'
        : /Mac OS X|Macintosh/.test(ua)
          ? 'macOS'
          : /Linux/.test(ua)
            ? 'Linux'
            : null;

  if (browser && system) return `${browser} · ${system}`;
  if (browser) return browser;
  return ua.length > MAX_RAW_LENGTH ? `${ua.slice(0, MAX_RAW_LENGTH - 3)}...` : ua;
}
