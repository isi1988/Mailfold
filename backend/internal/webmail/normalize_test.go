package webmail

import "testing"

func TestNormalizeAddress(t *testing.T) {
	cases := []struct {
		name  string
		email string
		want  string
	}{
		{"cyrillic domain", "noreply@родоскоп.рф", "noreply@xn--d1amkbbgbl.xn--p1ai"},
		{"already punycode", "noreply@xn--d1amkbbgbl.xn--p1ai", "noreply@xn--d1amkbbgbl.xn--p1ai"},
		{"already ascii", "user@example.com", "user@example.com"},
		{"cyrillic subdomain label", "user@mail.родоскоп.рф", "user@mail.xn--d1amkbbgbl.xn--p1ai"},
		{"no at sign", "not-an-email", "not-an-email"},
		{"empty string", "", ""},
		{"local part contains at sign, splits on the last one", "a@b@родоскоп.рф", "a@b@xn--d1amkbbgbl.xn--p1ai"},
		{"trailing at with empty domain", "user@", "user@"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeAddress(c.email)
			if got != c.want {
				t.Errorf("normalizeAddress(%q) = %q, want %q", c.email, got, c.want)
			}
		})
	}
}

func TestNormalizeAddressIsIdempotent(t *testing.T) {
	email := "noreply@родоскоп.рф"
	once := normalizeAddress(email)
	twice := normalizeAddress(once)
	if once != twice {
		t.Errorf("normalizeAddress should be idempotent: once=%q twice=%q", once, twice)
	}
}
