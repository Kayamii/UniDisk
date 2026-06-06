// Package pool orchestrates the unified storage pool: it routes new files to a
// provider account, streams bytes through to providers without buffering, and
// keeps the metadata store in sync. The pool is shared across all users; RBAC
// in the API layer governs who may upload/download/delete.
package pool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/unidisk/unidisk/internal/crypto"
	"github.com/unidisk/unidisk/internal/provider"
	"github.com/unidisk/unidisk/internal/store"
)

// Service ties the metadata store and provider registry together. It routes
// uploads round-robin across pool accounts under the fill threshold.
type Service struct {
	store    *store.Store
	registry *provider.Registry
	crypto   *crypto.Box

	// rrMu guards the round-robin cursor: the id of the account that received
	// the previous upload. In-memory and best-effort — resetting on restart
	// only restarts the rotation, which is harmless.
	rrMu     sync.Mutex
	rrCursor int64
}

// NewService builds the pool service.
func NewService(s *store.Store, r *provider.Registry, box *crypto.Box) *Service {
	return &Service{store: s, registry: r, crypto: box}
}

// providerFor resolves the provider implementation and decrypted credentials
// for an account.
func (s *Service) providerFor(a *store.Account) (provider.Provider, map[string]string, error) {
	p, err := s.registry.Get(a.Provider)
	if err != nil {
		return nil, nil, err
	}
	decrypted, err := s.crypto.Decrypt(a.CredentialsJSON)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt credentials: %w", err)
	}
	creds, err := provider.DecodeCreds(decrypted)
	if err != nil {
		return nil, nil, fmt.Errorf("decode credentials: %w", err)
	}
	return p, creds, nil
}

// Upload streams r into the pool: it routes to an account, pushes the bytes to
// that provider, and records the file's metadata. createdBy records the
// uploading user. The reader is consumed streaming — no full-body buffering.
func (s *Service) Upload(ctx context.Context, createdBy int64, parentID *int64, name string, size int64, mimeType string, r io.Reader) (*store.File, error) {
	accounts, err := s.store.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	s.rrMu.Lock()
	target, err := Route(accounts, settings.FillThresholdPct, size, s.rrCursor)
	if err == nil {
		s.rrCursor = target.ID
	}
	s.rrMu.Unlock()
	if err != nil {
		return nil, err
	}

	p, creds, err := s.providerFor(target)
	if err != nil {
		return nil, err
	}

	remoteID, err := p.Upload(ctx, creds, name, size, r)
	if err != nil {
		return nil, fmt.Errorf("provider upload: %w", err)
	}

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	f, err := s.store.CreateFile(ctx, &store.File{
		UserID:    createdBy,
		ParentID:  parentID,
		Name:      name,
		IsDir:     false,
		SizeBytes: max0(size),
		MimeType:  mimeType,
		AccountID: &target.ID,
		RemoteID:  &remoteID,
	})
	if err != nil {
		// Roll back the orphaned remote object so we don't leak storage.
		_ = p.Delete(ctx, creds, remoteID)
		return nil, fmt.Errorf("record file metadata: %w", err)
	}

	// Best-effort usage refresh so the dashboard reflects the new upload.
	s.refreshUsage(ctx, target, p, creds)
	return f, nil
}

// Download opens a streaming reader for a stored file. The caller closes it.
func (s *Service) Download(ctx context.Context, fileID int64) (*store.File, io.ReadCloser, error) {
	f, err := s.store.FileByID(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}
	if f.IsDir {
		return nil, nil, errors.New("cannot download a folder")
	}
	if f.AccountID == nil || f.RemoteID == nil {
		return nil, nil, errors.New("file has no backing storage")
	}
	acct, err := s.store.AccountByID(ctx, *f.AccountID)
	if err != nil {
		return nil, nil, err
	}
	p, creds, err := s.providerFor(acct)
	if err != nil {
		return nil, nil, err
	}
	rc, err := p.Download(ctx, creds, *f.RemoteID)
	if err != nil {
		return nil, nil, err
	}
	return f, rc, nil
}

// Delete removes a file's metadata and its bytes from the provider. Folders
// are deleted metadata-only (their children cascade); the MVP does not yet
// recurse into provider deletes for folder contents.
func (s *Service) Delete(ctx context.Context, fileID int64) error {
	f, err := s.store.FileByID(ctx, fileID)
	if err != nil {
		return err
	}
	if !f.IsDir && f.AccountID != nil && f.RemoteID != nil {
		if acct, err := s.store.AccountByID(ctx, *f.AccountID); err == nil {
			if p, creds, err := s.providerFor(acct); err == nil {
				// Best-effort: a provider-side failure shouldn't strand the
				// metadata, but we surface it if the metadata delete also fails.
				_ = p.Delete(ctx, creds, *f.RemoteID)
			}
		}
	}
	return s.store.DeleteFile(ctx, fileID)
}

func (s *Service) refreshUsage(ctx context.Context, a *store.Account, p provider.Provider, creds map[string]string) {
	u, err := p.Usage(ctx, creds)
	if err != nil {
		return
	}
	_ = s.store.UpdateAccountUsage(ctx, a.ID, u.QuotaBytes, u.UsedBytes)
}

func max0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
