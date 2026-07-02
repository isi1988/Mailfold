// Small formatting helpers shared across pages. mailcow encodes booleans as
// 0/1 ints and reports sizes in kilobytes.

export function isActive(v) {
  return v === 1 || v === true || v === '1';
}

// human renders a byte count as a compact size string.
export function human(bytes) {
  const n0 = Number(bytes) || 0;
  if (n0 <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let n = n0;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i += 1;
  }
  const rounded = n >= 10 || i === 0 ? Math.round(n) : Math.round(n * 10) / 10;
  return rounded + ' ' + units[i];
}

// humanKB renders a kilobyte count (mailcow's quota unit). 0 = unlimited.
export function humanKB(kb) {
  const n = Number(kb) || 0;
  if (n <= 0) return '∞';
  return human(n * 1024);
}

// asList tolerates mailcow returning {} or null for an empty collection.
export function asList(data) {
  return Array.isArray(data) ? data : [];
}
