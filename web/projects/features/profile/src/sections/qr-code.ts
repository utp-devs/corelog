import QRCode from 'qrcode';

/** QR modules of the text (error correction level M): `true` is a dark module. */
export function qrModules(text: string): boolean[][] {
  const { modules } = QRCode.create(text, { errorCorrectionLevel: 'M' });
  const size = modules.size;
  const rows: boolean[][] = [];
  for (let y = 0; y < size; y++) {
    const row: boolean[] = [];
    for (let x = 0; x < size; x++) row.push(Boolean(modules.get(y, x)));
    rows.push(row);
  }
  return rows;
}

/** SVG path (one rectangle per dark module, in a viewBox of size + 2 * quiet) for the matrix. */
export function qrPath(modules: readonly (readonly boolean[])[], quiet = 2): string {
  const parts: string[] = [];
  modules.forEach((row, y) => {
    row.forEach((dark, x) => {
      if (dark) parts.push(`M${x + quiet} ${y + quiet}h1v1h-1z`);
    });
  });
  return parts.join('');
}
