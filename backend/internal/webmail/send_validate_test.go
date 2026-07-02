package webmail

import "testing"

func TestOutgoingMessageValidate(t *testing.T) {
	ok := &OutgoingMessage{To: []string{"a@example.com"}, Subject: "Hello"}
	if err := ok.validate(); err != nil {
		t.Errorf("clean message should validate: %v", err)
	}

	cases := []*OutgoingMessage{
		{To: []string{"a@example.com"}, Subject: "Hi\r\nBcc: victim@example.com"},
		{To: []string{"a@example.com\r\nCc: victim@example.com"}, Subject: "Hi"},
		{Cc: []string{"c@example.com\n"}, To: []string{"a@example.com"}, Subject: "Hi"},
		{Bcc: []string{"b@example.com\r"}, To: []string{"a@example.com"}, Subject: "Hi"},
	}
	for i, m := range cases {
		if err := m.validate(); err == nil {
			t.Errorf("case %d: header injection should be rejected", i)
		}
	}
}
