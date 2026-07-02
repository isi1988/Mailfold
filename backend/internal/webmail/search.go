package webmail

import (
	"sort"

	"github.com/emersion/go-imap"
)

// Search returns the headers of messages in a folder matching a free-text query
// (IMAP TEXT search across headers and body), newest first.
func (c *Client) Search(email, password, folder, query string) ([]MessageHeader, error) {
	ic, err := c.dial(email, password)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ic.Logout() }()

	if _, err := ic.Select(folder, true); err != nil {
		return nil, err
	}

	criteria := imap.NewSearchCriteria()
	criteria.Text = []string{query}
	uids, err := ic.UidSearch(criteria)
	if err != nil {
		return nil, err
	}
	if len(uids) == 0 {
		return []MessageHeader{}, nil
	}
	// Cap to the most recent matches to bound the response size.
	if len(uids) > maxMessages {
		uids = uids[len(uids)-maxMessages:]
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchInternalDate, imap.FetchRFC822Size}
	ch := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() { done <- ic.UidFetch(seqset, items, ch) }()

	out := []MessageHeader{}
	for m := range ch {
		out = append(out, headerFrom(m))
	}
	if err := <-done; err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	return out, nil
}
