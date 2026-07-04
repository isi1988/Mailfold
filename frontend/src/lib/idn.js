// Decodes internationalized domain names (IDN) from their ASCII-compatible
// "punycode" form (e.g. "xn--d1amkbbgbl.xn--p1ai") back to Unicode (e.g.
// "родоскоп.рф") for DISPLAY ONLY.
//
// mailcow stores and returns every domain/mailbox identifier in its punycode
// form — that is what dovecot and postfix actually authenticate against, so
// the raw API value must never be altered before it flows into another
// request (a form submission, a <select> value, a chip that gets re-sent).
// These helpers exist purely to make that raw value readable in JSX text,
// table cells, toasts, and confirmation dialogs.
//
// The browser has no built-in punycode decoder (the URL API only ever
// produces the ASCII form), so this implements the RFC 3492 Bootstring decode
// procedure directly against the constants that section defines.

const BASE = 36;
const T_MIN = 1;
const T_MAX = 26;
const SKEW = 38;
const DAMP = 700;
const INITIAL_BIAS = 72;
const INITIAL_N = 128;
const DELIMITER = '-';
const ACE_PREFIX = 'xn--';

function adapt(delta, numPoints, firstTime) {
  let d = firstTime ? Math.floor(delta / DAMP) : delta >> 1;
  d += Math.floor(d / numPoints);
  let k = 0;
  while (d > ((BASE - T_MIN) * T_MAX) >> 1) {
    d = Math.floor(d / (BASE - T_MIN));
    k += BASE;
  }
  return k + Math.floor(((BASE - T_MIN + 1) * d) / (d + SKEW));
}

// basicToDigit maps an ASCII code point to its Bootstring digit value
// (0-25 for a-z/A-Z, 26-35 for 0-9), or BASE if it isn't a valid digit.
function basicToDigit(codePoint) {
  if (codePoint >= 0x30 && codePoint <= 0x39) return codePoint - 0x30 + 26;
  if (codePoint >= 0x41 && codePoint <= 0x5a) return codePoint - 0x41;
  if (codePoint >= 0x61 && codePoint <= 0x7a) return codePoint - 0x61;
  return BASE;
}

// decodeLabel decodes a single punycode-encoded label (with the "xn--" prefix
// already stripped) back to its Unicode code points. Returns null on any
// malformed input instead of throwing, since this only ever feeds a
// best-effort display.
function decodeLabel(input) {
  let n = INITIAL_N;
  let i = 0;
  let bias = INITIAL_BIAS;
  const output = [];

  let basicEnd = input.lastIndexOf(DELIMITER);
  if (basicEnd < 0) basicEnd = 0;
  for (let j = 0; j < basicEnd; j++) {
    const code = input.charCodeAt(j);
    if (code >= 0x80) return null; // the basic-code portion must be pure ASCII
    output.push(code);
  }

  let pos = basicEnd > 0 ? basicEnd + 1 : 0;
  const len = input.length;
  while (pos < len) {
    const oldI = i;
    let weight = 1;
    for (let k = BASE; ; k += BASE) {
      if (pos >= len) return null;
      const digit = basicToDigit(input.charCodeAt(pos++));
      if (digit >= BASE) return null;
      i += digit * weight;
      const t = k <= bias ? T_MIN : k >= bias + T_MAX ? T_MAX : k - bias;
      if (digit < t) break;
      weight *= BASE - t;
    }
    const numPoints = output.length + 1;
    bias = adapt(i - oldI, numPoints, oldI === 0);
    n += Math.floor(i / numPoints);
    i %= numPoints;
    if (i > output.length) return null; // malformed insertion index
    output.splice(i, 0, n);
    i++;
  }
  try {
    return String.fromCodePoint(...output);
  } catch {
    return null;
  }
}

function decodeDomainLabel(label) {
  if (!label.toLowerCase().startsWith(ACE_PREFIX)) return label;
  const decoded = decodeLabel(label.slice(ACE_PREFIX.length));
  return decoded == null ? label : decoded;
}

// decodeIdnDomain decodes every "xn--" label in a dot-separated domain name to
// Unicode. A domain with no ACE-prefixed label is returned unchanged.
export function decodeIdnDomain(domain) {
  if (!domain || domain.indexOf(ACE_PREFIX) === -1) return domain || '';
  return domain.split('.').map(decodeDomainLabel).join('.');
}

// decodeIdnAddress decodes the domain part of a "local@domain" email address
// for display, leaving the local part untouched (IDNA never applies there).
// A bare domain (no "@") is decoded the same way as decodeIdnDomain.
export function decodeIdnAddress(address) {
  if (!address) return address || '';
  const at = address.lastIndexOf('@');
  if (at < 0) return decodeIdnDomain(address);
  return address.slice(0, at + 1) + decodeIdnDomain(address.slice(at + 1));
}
