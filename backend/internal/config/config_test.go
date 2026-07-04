package config

import (
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("MAILFOLD_MAILCOW_URL", "https://mail.example")
	t.Setenv("MAILFOLD_MAILCOW_API_KEY", "key")
	t.Setenv("MAILFOLD_ADMIN_PASSWORD", "secret")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr=%q", cfg.Addr)
	}
	if cfg.AdminUser != "admin" {
		t.Errorf("AdminUser=%q", cfg.AdminUser)
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Errorf("SessionTTL=%v", cfg.SessionTTL)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Errorf("CORSOrigins=%v", cfg.CORSOrigins)
	}
	if cfg.MailcowInsecureTLS {
		t.Error("MailcowInsecureTLS should default to false")
	}
	if cfg.LoginRateMax != 5 || cfg.LoginRateWindow != time.Minute || cfg.MaxBodyBytes != 25<<20 {
		t.Errorf("rate/body defaults wrong: max=%d window=%v body=%d",
			cfg.LoginRateMax, cfg.LoginRateWindow, cfg.MaxBodyBytes)
	}
}

func TestLoadOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("MAILFOLD_ADDR", ":9000")
	t.Setenv("MAILFOLD_ADMIN_USER", "root")
	t.Setenv("MAILFOLD_MAILCOW_INSECURE_TLS", "true")
	t.Setenv("MAILFOLD_SESSION_TTL", "1h")
	t.Setenv("MAILFOLD_CORS_ORIGINS", "https://a.com, https://b.com ,")
	t.Setenv("MAILFOLD_LOGIN_RATE_MAX", "10")
	t.Setenv("MAILFOLD_LOGIN_RATE_WINDOW", "30s")
	t.Setenv("MAILFOLD_MAX_BODY_BYTES", "2048")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9000" || cfg.AdminUser != "root" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if cfg.LoginRateMax != 10 || cfg.LoginRateWindow != 30*time.Second || cfg.MaxBodyBytes != 2048 {
		t.Errorf("rate/body overrides wrong: max=%d window=%v body=%d",
			cfg.LoginRateMax, cfg.LoginRateWindow, cfg.MaxBodyBytes)
	}
	if !cfg.MailcowInsecureTLS {
		t.Error("expected MailcowInsecureTLS true")
	}
	if cfg.SessionTTL != time.Hour {
		t.Errorf("SessionTTL=%v", cfg.SessionTTL)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Errorf("CORSOrigins=%v", cfg.CORSOrigins)
	}
}

func TestLoadInvalidFallbacks(t *testing.T) {
	setRequired(t)
	t.Setenv("MAILFOLD_MAILCOW_INSECURE_TLS", "notabool")
	t.Setenv("MAILFOLD_SESSION_TTL", "notaduration")
	t.Setenv("MAILFOLD_CORS_ORIGINS", "  ,  ")
	t.Setenv("MAILFOLD_MAX_BODY_BYTES", "notanint")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxBodyBytes != 25<<20 {
		t.Errorf("bad int should fall back to default: %d", cfg.MaxBodyBytes)
	}
	if cfg.MailcowInsecureTLS {
		t.Error("bad bool should fall back to false")
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Errorf("bad duration should fall back: %v", cfg.SessionTTL)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Errorf("blank CORS should fall back: %v", cfg.CORSOrigins)
	}
}

func TestLoadAdminEncKeyAbsent(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AdminEncKey != nil {
		t.Errorf("AdminEncKey should default to nil, got %v", cfg.AdminEncKey)
	}
}

func TestLoadAdminEncKeyValidHex(t *testing.T) {
	setRequired(t)
	t.Setenv("MAILFOLD_ADMIN_ENC_KEY", "1111111111111111111111111111111111111111111111111111111111111111")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.AdminEncKey) < 32 {
		t.Errorf("AdminEncKey too short: %d bytes", len(cfg.AdminEncKey))
	}
}

func TestLoadAdminEncKeyValidBase64(t *testing.T) {
	setRequired(t)
	// 32 raw bytes, standard base64.
	t.Setenv("MAILFOLD_ADMIN_ENC_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.AdminEncKey) < 32 {
		t.Errorf("AdminEncKey too short: %d bytes", len(cfg.AdminEncKey))
	}
}

func TestLoadAdminEncKeyInvalid(t *testing.T) {
	setRequired(t)
	t.Setenv("MAILFOLD_ADMIN_ENC_KEY", "too-short")
	if _, err := Load(); err == nil {
		t.Error("expected an error for an undecodable/too-short admin enc key")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	cases := []struct{ name, url, key, pass string }{
		{"no url", "", "k", "p"},
		{"no key", "https://u", "", "p"},
		{"no password", "https://u", "k", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("MAILFOLD_MAILCOW_URL", c.url)
			t.Setenv("MAILFOLD_MAILCOW_API_KEY", c.key)
			t.Setenv("MAILFOLD_ADMIN_PASSWORD", c.pass)
			if _, err := Load(); err == nil {
				t.Error("expected error for missing required value")
			}
		})
	}
}
