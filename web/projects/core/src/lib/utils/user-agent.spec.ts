import { describeUserAgent } from './user-agent';

describe('describeUserAgent', () => {
  it('reduces a desktop browser string to browser and system', () => {
    expect(
      describeUserAgent(
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36',
      ),
    ).toBe('Chrome · Windows');
    expect(
      describeUserAgent(
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15',
      ),
    ).toBe('Safari · macOS');
  });

  it('recognises a headless browser as Chrome', () => {
    expect(
      describeUserAgent(
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) HeadlessChrome/159.0 Safari/537.36',
      ),
    ).toBe('Chrome · Windows');
  });

  it('names mobile systems and Edge before Chrome', () => {
    expect(
      describeUserAgent('Mozilla/5.0 (Linux; Android 14) Chrome/130 Mobile Safari/537.36'),
    ).toBe('Chrome · Android');
    expect(
      describeUserAgent('Mozilla/5.0 (Windows NT 10.0) Chrome/130 Safari/537.36 Edg/130.0'),
    ).toBe('Edge · Windows');
  });

  it('keeps unknown clients readable and handles an empty value', () => {
    expect(describeUserAgent('curl/8.21.0')).toBe('curl/8.21.0');
    expect(describeUserAgent('')).toBe('Navegador sin identificar');
    expect(describeUserAgent('x'.repeat(100))).toHaveLength(60);
  });
});
