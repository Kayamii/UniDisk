// Package s3 implements the storage provider interface for any S3-compatible
// object store: Amazon S3, Backblaze B2, Wasabi, Cloudflare R2, MinIO,
// DigitalOcean Spaces, etc. A single implementation covers them all via a
// configurable endpoint. Credentials are entered manually in the dashboard
// (access key / secret / bucket / region / endpoint) — there is no OAuth.
package s3

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/unidisk/unidisk/internal/provider"
)

// Provider implements provider.Provider for S3-compatible stores. It does NOT
// implement OAuthProvider — it uses the manual credential form.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string  { return "s3" }
func (p *Provider) Title() string { return "S3-compatible" }

func (p *Provider) CredentialSchema() []provider.CredentialField {
	return []provider.CredentialField{
		{Key: "access_key_id", Label: "Access Key ID", Type: provider.FieldText, Required: true},
		{Key: "secret_access_key", Label: "Secret Access Key", Type: provider.FieldPassword, Required: true},
		{Key: "bucket", Label: "Bucket", Type: provider.FieldText, Required: true},
		{Key: "region", Label: "Region", Type: provider.FieldText, Required: true,
			Help: "e.g. us-east-1. For Backblaze/Wasabi/R2 use their region."},
		{Key: "endpoint", Label: "Endpoint URL", Type: provider.FieldText, Required: false,
			Help: "Leave blank for AWS S3. For others, e.g. https://s3.us-west-001.backblazeb2.com"},
	}
}

// clientFor builds an S3 client from the account credentials, honoring a custom
// endpoint and forcing path-style addressing (needed by MinIO/Backblaze/etc.).
func (p *Provider) clientFor(ctx context.Context, creds map[string]string) (*awss3.Client, string, error) {
	bucket := creds["bucket"]
	if creds["access_key_id"] == "" || creds["secret_access_key"] == "" || bucket == "" || creds["region"] == "" {
		return nil, "", fmt.Errorf("missing s3 credentials")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(creds["region"]),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds["access_key_id"], creds["secret_access_key"], "")),
	)
	if err != nil {
		return nil, "", err
	}
	client := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		if ep := creds["endpoint"]; ep != "" {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = true // required by most non-AWS S3 implementations
		}
	})
	return client, bucket, nil
}

func (p *Provider) Verify(ctx context.Context, creds map[string]string) (provider.VerifyResult, map[string]string, error) {
	client, bucket, err := p.clientFor(ctx, creds)
	if err != nil {
		return provider.VerifyResult{}, nil, err
	}
	// HeadBucket confirms the credentials and bucket access without listing.
	if _, err := client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: &bucket}); err != nil {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify s3: %w", err)
	}
	// Object stores don't report a fixed quota; show used = 0, quota = 0
	// (treated as "unlimited" by the router).
	return provider.VerifyResult{
		DisplayName: bucket,
		Usage:       provider.Usage{QuotaBytes: 0, UsedBytes: 0},
	}, creds, nil
}

func (p *Provider) Usage(ctx context.Context, creds map[string]string) (provider.Usage, error) {
	// S3 has no cheap usage call; report unlimited. (A future enhancement
	// could sum object sizes, but that's expensive on large buckets.)
	return provider.Usage{QuotaBytes: 0, UsedBytes: 0}, nil
}

func (p *Provider) Upload(ctx context.Context, creds map[string]string, name string, _ int64, r io.Reader) (string, error) {
	client, bucket, err := p.clientFor(ctx, creds)
	if err != nil {
		return "", err
	}
	// Unique key so identical filenames don't collide in the bucket; the
	// original name is preserved for display via UniDisk's own metadata.
	key := path.Join(randomPrefix(), name)
	uploader := manager.NewUploader(client)
	_, err = uploader.Upload(ctx, &awss3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   r,
	})
	if err != nil {
		return "", fmt.Errorf("upload to s3: %w", err)
	}
	return key, nil
}

func (p *Provider) Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error) {
	client, bucket, err := p.clientFor(ctx, creds)
	if err != nil {
		return nil, err
	}
	out, err := client.GetObject(ctx, &awss3.GetObjectInput{Bucket: &bucket, Key: &remoteID})
	if err != nil {
		return nil, fmt.Errorf("download from s3: %w", err)
	}
	return out.Body, nil
}

func (p *Provider) Delete(ctx context.Context, creds map[string]string, remoteID string) error {
	client, bucket, err := p.clientFor(ctx, creds)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &bucket, Key: &remoteID})
	if err != nil {
		return fmt.Errorf("delete from s3: %w", err)
	}
	return nil
}

// randomPrefix returns an 8-char hex prefix to namespace uploaded objects.
func randomPrefix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
