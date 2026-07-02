package mailcow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFlexInt64Unmarshal(t *testing.T) {
	cases := map[string]int64{
		`123`:       123,
		`"456"`:     456,
		`0`:         0,
		`"0"`:       0,
		`""`:        0,
		`null`:      0,
		`-9`:        -9,
		`"1048576"`: 1048576,
	}
	for in, want := range cases {
		var f FlexInt64
		if err := json.Unmarshal([]byte(in), &f); err != nil {
			t.Errorf("Unmarshal(%s): unexpected error %v", in, err)
			continue
		}
		if int64(f) != want {
			t.Errorf("Unmarshal(%s) = %d, want %d", in, int64(f), want)
		}
	}

	var bad FlexInt64
	if err := json.Unmarshal([]byte(`"not a number"`), &bad); err == nil {
		t.Error("expected an error for a non-numeric string")
	}
}

func TestFlexInt64InDomain(t *testing.T) {
	// mailcow returns byte counts as quoted strings; the Domain must still decode.
	const body = `{"domain_name":"x.io","bytes_total":"2097152","max_quota_for_domain":"10737418240","mboxes_in_domain":3}`
	var d Domain
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("decode domain: %v", err)
	}
	if d.QuotaUsed != 2097152 || d.MaxQuota != 10737418240 || d.Mailboxes != 3 {
		t.Errorf("decoded domain wrong: %+v", d)
	}
	// It marshals back out as a plain number, not a string.
	out, _ := json.Marshal(d)
	if got := string(out); !strings.Contains(got, `"bytes_total":2097152`) {
		t.Errorf("FlexInt64 should marshal as a number, got %s", got)
	}
}
