import { createHmac } from 'node:crypto';

const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

/** Bytes of a base32 text (RFC 4648, spaces and padding ignored). */
export function base32Decode(text: string): Buffer {
  const clean = text.replace(/[\s=]/g, '').toUpperCase();
  let bits = 0;
  let value = 0;
  const bytes: number[] = [];
  for (const char of clean) {
    const index = ALPHABET.indexOf(char);
    if (index < 0) throw new Error(`not a base32 character: ${char}`);
    value = (value << 5) | index;
    bits += 5;
    if (bits >= 8) {
      bytes.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  return Buffer.from(bytes);
}

/**
 * Six-digit TOTP code (RFC 6238: HMAC-SHA1, 30 second step) for a base32 secret.
 * `offsetSteps` shifts the time window, for example 1 for the next code.
 */
export function totp(secret: string, now = Date.now(), offsetSteps = 0): string {
  const counter = Math.floor(now / 1000 / 30) + offsetSteps;
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac('sha1', base32Decode(secret)).update(message).digest();
  const offset = digest[digest.length - 1] & 0x0f;
  const binary =
    ((digest[offset] & 0x7f) << 24) |
    (digest[offset + 1] << 16) |
    (digest[offset + 2] << 8) |
    digest[offset + 3];
  return String(binary % 1_000_000).padStart(6, '0');
}
