package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// maxBulkMailboxRows bounds how many mailboxes a single CSV import may create,
// so one oversized upload cannot tie up the request for an unbounded time or
// hammer mailcow with an unbounded number of add calls.
const maxBulkMailboxRows = 500

// defaultBulkQuotaGB is applied when a row omits (or has an invalid) quota_gb
// column, matching the create-mailbox drawer's own default.
const defaultBulkQuotaGB = 3

// registerMailboxBulkRoutes attaches the CSV bulk-import endpoint.
func (s *Server) registerMailboxBulkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mailboxes/bulk", s.requireAuth(s.handleMailboxBulkImport))
}

type mailboxBulkRequest struct {
	CSV string `json:"csv"`
}

// mailboxBulkRowResult reports the outcome of importing one CSV row.
type mailboxBulkRowResult struct {
	Row     int    `json:"row"` // 1-based, counting only data rows (the header is row 0)
	Mailbox string `json:"mailbox"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

type mailboxBulkResponse struct {
	Results []mailboxBulkRowResult `json:"results"`
	Created int                    `json:"created"`
	Failed  int                    `json:"failed"`
}

// handleMailboxBulkImport creates one mailbox per CSV row. The CSV must have a
// header naming its columns (order-independent); local_part, domain and
// password are required, name/quota_gb/active are optional. Every row is
// attempted independently — one bad row does not abort the rest — and the
// response reports a per-row outcome so the caller can see exactly which
// addresses were created and which failed, and why.
func (s *Server) handleMailboxBulkImport(w http.ResponseWriter, r *http.Request) {
	var req mailboxBulkRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	rows, err := parseMailboxBulkCSV(req.CSV)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the CSV has no data rows"})
		return
	}
	if len(rows) > maxBulkMailboxRows {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("too many rows (max %d per import)", maxBulkMailboxRows)})
		return
	}

	resp := mailboxBulkResponse{Results: make([]mailboxBulkRowResult, 0, len(rows))}
	for i, row := range rows {
		result := s.importMailboxRow(r.Context(), i+1, row)
		resp.Results = append(resp.Results, result)
		if result.OK {
			resp.Created++
		} else {
			resp.Failed++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// importMailboxRow validates and creates a single mailbox, translating any
// problem (missing field, bad quota, upstream failure) into a row result
// rather than aborting the whole import.
func (s *Server) importMailboxRow(ctx context.Context, rowNum int, row map[string]string) mailboxBulkRowResult {
	localPart := strings.TrimSpace(row["local_part"])
	domain := strings.TrimSpace(row["domain"])
	password := row["password"]
	mailbox := localPart + "@" + domain

	if localPart == "" || domain == "" {
		return mailboxBulkRowResult{Row: rowNum, Mailbox: mailbox, Error: "local_part and domain are required"}
	}
	if len(password) < 8 {
		return mailboxBulkRowResult{Row: rowNum, Mailbox: mailbox, Error: "password must be at least 8 characters"}
	}

	quotaGB := defaultBulkQuotaGB
	if raw := strings.TrimSpace(row["quota_gb"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return mailboxBulkRowResult{Row: rowNum, Mailbox: mailbox, Error: "quota_gb must be a positive integer"}
		}
		quotaGB = n
	}

	active := "1"
	if raw := strings.TrimSpace(row["active"]); raw != "" && (raw == "0" || strings.EqualFold(raw, "false")) {
		active = "0"
	}

	attr := map[string]any{
		"local_part": localPart,
		"domain":     domain,
		"name":       row["name"],
		"quota":      quotaGB * 1024,
		"password":   password,
		"password2":  password,
		"active":     active,
	}
	results, err := s.mc.AddMailbox(ctx, attr)
	if err != nil {
		return mailboxBulkRowResult{Row: rowNum, Mailbox: mailbox, Error: err.Error()}
	}
	ok, msg := mailcow.ResultsOK(results)
	return mailboxBulkRowResult{Row: rowNum, Mailbox: mailbox, OK: ok, Error: okErrString(ok, msg)}
}

// okErrString returns msg only when the operation failed, so a successful row
// result never carries a stale/irrelevant "info" message in its Error field.
func okErrString(ok bool, msg string) string {
	if ok {
		return ""
	}
	return msg
}

// parseMailboxBulkCSV reads a header-driven CSV into a slice of column-name to
// cell-value maps, one per data row. Column names are matched case- and
// whitespace-insensitively so "Local_Part" or " domain " work the same as
// "local_part"/"domain". Ragged rows (wrong field count) are rejected outright
// rather than silently misaligning columns.
func parseMailboxBulkCSV(raw string) ([]map[string]string, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("the CSV is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the CSV header: %w", err)
	}
	cols := make([]string, len(header))
	for i, h := range header {
		cols[i] = strings.ToLower(strings.TrimSpace(h))
	}

	var rows []map[string]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not read a CSV row: %w", err)
		}
		row := make(map[string]string, len(cols))
		for i, col := range cols {
			if i < len(record) {
				row[col] = record[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
