// Browser-side glue for the WebAuthn/passkey ceremonies. The backend's JSON
// (see internal/api/webauthn.go) carries every binary field — challenge,
// user.id, credential ids — as base64url text (the go-webauthn library's own
// wire format), while the browser's navigator.credentials API deals in
// ArrayBuffers. This module is the only place that conversion happens, so
// every caller (Settings' passkey enrollment, the login screen's passkey
// step) just passes/receives plain JSON.

function base64UrlToBuffer(b64url) {
  const padded = b64url + '='.repeat((4 - (b64url.length % 4)) % 4);
  const base64 = padded.replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes.buffer;
}

function bufferToBase64Url(buf) {
  const bytes = new Uint8Array(buf);
  let str = '';
  for (let i = 0; i < bytes.length; i++) str += String.fromCharCode(bytes[i]);
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// isWebAuthnSupported reports whether this browser can attempt a passkey
// ceremony at all, so the UI can hide the feature entirely rather than offer
// a button that would just fail.
export function isWebAuthnSupported() {
  return typeof window !== 'undefined' && !!window.PublicKeyCredential && !!navigator.credentials;
}

function decodeCreationOptions(json) {
  const opts = json.publicKey;
  return {
    ...opts,
    challenge: base64UrlToBuffer(opts.challenge),
    user: { ...opts.user, id: base64UrlToBuffer(opts.user.id) },
    excludeCredentials: (opts.excludeCredentials || []).map(c => ({ ...c, id: base64UrlToBuffer(c.id) })),
  };
}

function decodeRequestOptions(json) {
  const opts = json.publicKey;
  return {
    ...opts,
    challenge: base64UrlToBuffer(opts.challenge),
    allowCredentials: (opts.allowCredentials || []).map(c => ({ ...c, id: base64UrlToBuffer(c.id) })),
  };
}

function encodeAttestation(cred) {
  const r = cred.response;
  return {
    id: cred.id,
    rawId: bufferToBase64Url(cred.rawId),
    type: cred.type,
    response: {
      attestationObject: bufferToBase64Url(r.attestationObject),
      clientDataJSON: bufferToBase64Url(r.clientDataJSON),
      transports: typeof r.getTransports === 'function' ? r.getTransports() : [],
    },
    clientExtensionResults: typeof cred.getClientExtensionResults === 'function' ? cred.getClientExtensionResults() : {},
  };
}

function encodeAssertion(cred) {
  const r = cred.response;
  return {
    id: cred.id,
    rawId: bufferToBase64Url(cred.rawId),
    type: cred.type,
    response: {
      authenticatorData: bufferToBase64Url(r.authenticatorData),
      clientDataJSON: bufferToBase64Url(r.clientDataJSON),
      signature: bufferToBase64Url(r.signature),
      userHandle: r.userHandle ? bufferToBase64Url(r.userHandle) : null,
    },
    clientExtensionResults: typeof cred.getClientExtensionResults === 'function' ? cred.getClientExtensionResults() : {},
  };
}

// createPasskey runs navigator.credentials.create() against the server's
// creation options (as returned by POST /api/account/webauthn/register/begin)
// and returns the JSON body /register/finish expects.
export async function createPasskey(creationOptionsJSON) {
  const publicKey = decodeCreationOptions(creationOptionsJSON);
  const cred = await navigator.credentials.create({ publicKey });
  return encodeAttestation(cred);
}

// getPasskeyAssertion runs navigator.credentials.get() against the server's
// request options (as returned by POST /api/auth/2fa/webauthn/begin) and
// returns the JSON body /api/auth/2fa/webauthn/finish expects.
export async function getPasskeyAssertion(requestOptionsJSON) {
  const publicKey = decodeRequestOptions(requestOptionsJSON);
  const cred = await navigator.credentials.get({ publicKey });
  return encodeAssertion(cred);
}
