package webmail

import (
	"strings"

	"golang.org/x/net/idna"
)

// normalizeAddress rewrites the domain part of an email address to its
// ASCII-compatible ("punycode") form when it contains non-ASCII characters.
// This is not cosmetic: mailcow's dovecot and postfix store and authenticate
// internationalized domains ONLY under this form — confirmed live against a
// production mailcow (`doveadm user 'noreply@xn--d1amkbbgbl.xn--p1ai'` finds
// the account; `doveadm user 'noreply@родоскоп.рф'` reports "doesn't exist").
// Without this, a mailbox on an IDN domain could never log in or send unless
// the caller already knew to type the ugly xn-- form by hand.
//
// The local part is left untouched — IDNA only ever applies to domain labels.
// Malformed input (no "@", or a domain idna can't encode) is returned
// unchanged so the original error surfaces naturally from the failed
// IMAP/SMTP login rather than being masked here. Called at the single dial
// point for IMAP (Client.dial) and the send path for SMTP (Client.Send,
// Client.SaveToSent), so every public method funnels through one of these two
// normalized forms.
func normalizeAddress(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return email
	}
	local, domain := email[:at], email[at+1:]
	ascii, err := idna.ToASCII(domain)
	if err != nil || ascii == "" {
		return email
	}
	return local + "@" + ascii
}
