package store

import "context"

// Settings holds the instance-wide routing preferences (single row, id = 1).
type Settings struct {
	FillThresholdPct int `json:"fill_threshold_pct"`
}

// GetSettings returns the instance routing settings.
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var out Settings
	err := s.db.QueryRowContext(ctx,
		`SELECT fill_threshold_pct FROM settings WHERE id = 1`).
		Scan(&out.FillThresholdPct)
	return out, err
}

// UpdateSettings updates the instance routing settings.
func (s *Store) UpdateSettings(ctx context.Context, set Settings) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE settings SET fill_threshold_pct = ? WHERE id = 1`, set.FillThresholdPct)
	return err
}
