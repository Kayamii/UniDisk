package pool

import (
	"errors"

	"github.com/unidisk/unidisk/internal/store"
)

// ErrNoCapacity is returned when no active account can hold a new file.
var ErrNoCapacity = errors.New("no storage account has enough available space")

// available returns an account's free bytes. A quota of 0 means
// unknown/unlimited, which we treat as effectively boundless.
func available(a *store.Account) int64 {
	if a.QuotaBytes == 0 {
		return 1<<62 - 1
	}
	free := a.QuotaBytes - a.UsedBytes
	if free < 0 {
		return 0
	}
	return free
}

// usedPct returns an account's percent-used (0–100). Unlimited accounts
// (quota 0) report 0, so they never trip the fill threshold.
func usedPct(a *store.Account) int {
	if a.QuotaBytes <= 0 {
		return 0
	}
	return int(a.UsedBytes * 100 / a.QuotaBytes)
}

// fits reports whether an active account can hold a file of sizeBytes. A size
// of -1 (unknown) only requires the account be active.
func fits(a *store.Account, sizeBytes int64) bool {
	if a.Status != "active" {
		return false
	}
	return sizeBytes < 0 || available(a) >= sizeBytes
}

// Route selects the account that receives a new file using a round-robin
// policy with a soft fill cap:
//
//   - Prefer accounts UNDER thresholdPct used, chosen round-robin (rotating
//     from the account after the last one used) so uploads spread evenly.
//   - If EVERY fitting account is at/above thresholdPct, fall back to the one
//     with the most free space in absolute bytes.
//
// lastID is the id of the account used for the previous upload (0 if none);
// it advances the round-robin cursor. accounts must be a stable, ordered slice
// (the store returns them ordered by priority, id).
func Route(accounts []*store.Account, thresholdPct int, sizeBytes int64, lastID int64) (*store.Account, error) {
	// Candidates that can physically hold the file.
	var fitting []*store.Account
	for _, a := range accounts {
		if fits(a, sizeBytes) {
			fitting = append(fitting, a)
		}
	}
	if len(fitting) == 0 {
		return nil, ErrNoCapacity
	}

	// Primary pool: under the threshold. Round-robin among these.
	var under []*store.Account
	for _, a := range fitting {
		if usedPct(a) < thresholdPct {
			under = append(under, a)
		}
	}
	if len(under) > 0 {
		return roundRobin(under, lastID), nil
	}

	// Fallback: all candidates are at/above the threshold — pick the most free.
	best := fitting[0]
	for _, a := range fitting[1:] {
		if available(a) > available(best) {
			best = a
		}
	}
	return best, nil
}

// roundRobin returns the account after lastID in the slice, wrapping around.
// If lastID isn't present (e.g. that account was deleted or this is the first
// upload), it starts at the beginning.
func roundRobin(pool []*store.Account, lastID int64) *store.Account {
	start := 0
	for i, a := range pool {
		if a.ID == lastID {
			start = (i + 1) % len(pool)
			break
		}
	}
	return pool[start]
}
