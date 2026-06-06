package pool

import (
	"testing"

	"github.com/unidisk/unidisk/internal/store"
)

func acct(id int64, quota, used int64) *store.Account {
	return &store.Account{ID: id, Status: "active", QuotaBytes: quota, UsedBytes: used}
}

// Under the threshold, uploads should rotate round-robin across accounts.
func TestRoute_RoundRobinUnderThreshold(t *testing.T) {
	accts := []*store.Account{
		acct(1, 100, 10), // 10% used
		acct(2, 100, 20), // 20% used
		acct(3, 100, 30), // 30% used
	}
	// Start with no prior (lastID 0) → should pick first; then follow each
	// choice forward to confirm rotation 1→2→3→1.
	want := []int64{1, 2, 3, 1, 2}
	last := int64(0)
	for i, w := range want {
		got, err := Route(accts, 80, 1, last)
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if got.ID != w {
			t.Fatalf("step %d: got account %d, want %d", i, got.ID, w)
		}
		last = got.ID
	}
}

// Accounts at/above the threshold are skipped while others remain under it.
func TestRoute_SkipsOverThreshold(t *testing.T) {
	accts := []*store.Account{
		acct(1, 100, 90), // 90% — over 80, skip
		acct(2, 100, 10), // 10% — under, eligible
	}
	for i := 0; i < 4; i++ {
		got, err := Route(accts, 80, 1, got_or(accts, i))
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != 2 {
			t.Fatalf("iter %d: expected only account 2, got %d", i, got.ID)
		}
	}
}

// When EVERY account is over the threshold, fall back to the most free space
// in absolute bytes — the user's example: disk1 1GB free vs disk2 2GB free.
func TestRoute_FallbackMostFreeBytes(t *testing.T) {
	accts := []*store.Account{
		acct(1, 15, 14), // 93% used, 1 unit free
		acct(2, 32, 30), // 94% used, 2 units free  ← most free, should win
	}
	got, err := Route(accts, 80, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 {
		t.Fatalf("fallback should pick most-free (2), got %d", got.ID)
	}
}

// A file larger than any free space yields ErrNoCapacity.
func TestRoute_NoCapacity(t *testing.T) {
	accts := []*store.Account{acct(1, 100, 99)}
	if _, err := Route(accts, 80, 50, 0); err != ErrNoCapacity {
		t.Fatalf("expected ErrNoCapacity, got %v", err)
	}
}

// Unlimited accounts (quota 0) never trip the threshold.
func TestRoute_UnlimitedNeverFull(t *testing.T) {
	accts := []*store.Account{acct(1, 0, 0)}
	got, err := Route(accts, 80, -1, 0)
	if err != nil || got.ID != 1 {
		t.Fatalf("unlimited account should be eligible, got %v err %v", got, err)
	}
}

// helper: irrelevant cursor for the skip test (always returns 0-ish).
func got_or(_ []*store.Account, _ int) int64 { return 0 }
