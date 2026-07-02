package config

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestAPIKeyDisabledByDefault(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKeyEnabled {
		t.Error("API keys should be off by default")
	}
	if cfg.APIKeyMasterKey != nil {
		t.Error("no master key should be loaded when disabled")
	}
	if cfg.APIKeyRateMax != 120 || cfg.APIKeyMaxRecipients != 50 {
		t.Errorf("unexpected defaults: rate=%d recip=%d", cfg.APIKeyRateMax, cfg.APIKeyMaxRecipients)
	}
}

func TestAPIKeyEnabledRequiresMasterKey(t *testing.T) {
	setRequired(t)
	t.Setenv("MAILFOLD_APIKEY_ENABLED", "true")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MAILFOLD_APIKEY_MASTER_KEY") {
		t.Fatalf("expected a master-key requirement error, got %v", err)
	}
}

func TestAPIKeyMasterKeyDecoding(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	cases := map[string]string{
		"hex":       hex.EncodeToString(raw),
		"base64std": base64.StdEncoding.EncodeToString(raw),
		"base64raw": base64.RawStdEncoding.EncodeToString(raw),
	}
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			setRequired(t)
			t.Setenv("MAILFOLD_APIKEY_ENABLED", "true")
			t.Setenv("MAILFOLD_APIKEY_MASTER_KEY", encoded)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(cfg.APIKeyMasterKey) < 32 {
				t.Errorf("master key too short: %d", len(cfg.APIKeyMasterKey))
			}
		})
	}

	t.Run("too-short", func(t *testing.T) {
		setRequired(t)
		t.Setenv("MAILFOLD_APIKEY_ENABLED", "true")
		t.Setenv("MAILFOLD_APIKEY_MASTER_KEY", hex.EncodeToString([]byte("short")))
		if _, err := Load(); err == nil {
			t.Error("a <32-byte key must be rejected")
		}
	})
}
