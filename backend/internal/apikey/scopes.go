package apikey

import "strings"

// The scope set. Scopes are a second, finer gate on top of the mailcow app
// password's protocol scoping (which is already IMAP+SMTP only), so a read-only
// key physically cannot mutate or destroy mail.
const (
	// ScopeSend authorizes SMTP submission as the bound mailbox.
	ScopeSend = "mail:send"
	// ScopeRead authorizes the collect/read verbs (folders, messages, message,
	// search, attachment).
	ScopeRead = "mail:read"
	// ScopeWrite authorizes mutation (flag, delete). Kept distinct from read so a
	// pure collector key can be strictly read-only.
	ScopeWrite = "mail:write"
)

// DefaultScopes is applied when a key is minted without an explicit scope list —
// a key that just works. Callers narrow it (for example to read-only) as needed.
func DefaultScopes() []string {
	return []string{ScopeSend, ScopeRead, ScopeWrite}
}

// ValidScope reports whether s is a recognised scope.
func ValidScope(s string) bool {
	switch s {
	case ScopeSend, ScopeRead, ScopeWrite:
		return true
	default:
		return false
	}
}

// JoinScopes serialises a scope list for storage (comma-separated).
func JoinScopes(scopes []string) string {
	return strings.Join(scopes, ",")
}

// SplitScopes parses a stored scope string back into a slice, dropping blanks.
func SplitScopes(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// HasScope reports whether the comma-joined scope string grants want.
func HasScope(stored, want string) bool {
	for _, s := range SplitScopes(stored) {
		if s == want {
			return true
		}
	}
	return false
}
