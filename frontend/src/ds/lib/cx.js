// Tiny className joiner — filters falsy, joins with spaces.
export function cx(...parts) {
  return parts.filter(Boolean).join(' ');
}
