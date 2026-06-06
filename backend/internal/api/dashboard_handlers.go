package api

import "net/http"

// poolStats is the storage-overview payload for the dashboard.
type poolStats struct {
	TotalBytes     int64            `json:"total_bytes"`
	UsedBytes      int64            `json:"used_bytes"`
	AvailableBytes int64            `json:"available_bytes"`
	AccountCount   int              `json:"account_count"`
	Accounts       []accountSummary `json:"accounts"`
}

type accountSummary struct {
	ID          int64  `json:"id"`
	Provider    string `json:"provider"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	QuotaBytes  int64  `json:"quota_bytes"`
	UsedBytes   int64  `json:"used_bytes"`
}

// handleStats aggregates every connected account into a single pool view —
// the "Total / Used / Available" figures from the dashboard spec.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load stats")
		return
	}
	stats := poolStats{AccountCount: len(accounts), Accounts: []accountSummary{}}
	for _, a := range accounts {
		stats.TotalBytes += a.QuotaBytes
		stats.UsedBytes += a.UsedBytes
		stats.Accounts = append(stats.Accounts, accountSummary{
			ID: a.ID, Provider: a.Provider, DisplayName: a.DisplayName,
			Status: a.Status, QuotaBytes: a.QuotaBytes, UsedBytes: a.UsedBytes,
		})
	}
	stats.AvailableBytes = stats.TotalBytes - stats.UsedBytes
	if stats.AvailableBytes < 0 {
		stats.AvailableBytes = 0
	}
	writeJSON(w, http.StatusOK, stats)
}
