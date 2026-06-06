# UniDisk

**Self-hosted storage aggregation platform.**

Connect multiple cloud storage accounts and use them as a single virtual storage pool.

UniDisk allows users to combine Google Drive, OneDrive, Dropbox, S3, NAS storage, and other providers into one unified filesystem accessible through a web dashboard and API.

Instead of managing multiple storage accounts separately, users interact with a single storage space while UniDisk handles provider selection and file placement.

---

## Self-Hosted

UniDisk runs entirely on your infrastructure.

Deploy using Docker, Docker Compose, Kubernetes, or a Linux server.

Your files remain stored in your connected providers.

UniDisk only manages metadata and storage orchestration.

---

## Example

Connect:

* Google Drive Account #1 (15 GB)
* Google Drive Account #2 (15 GB)
* OneDrive (5 GB)

UniDisk automatically creates:

Storage Pool

* Total Capacity: 35 GB
* Used: 12 GB
* Available: 23 GB

Users interact with a single filesystem without caring where files are physically stored.

---

## Key Features

### Unified Storage Pool

Aggregate multiple storage providers into one logical storage space.

### Multi-Account Support

Connect unlimited accounts from the same provider.

Examples:

* Multiple Google accounts
* Multiple Microsoft accounts
* Multiple Dropbox accounts

### File Manager

* Upload files
* Download files
* Create folders
* Move files
* Rename files
* Delete files
* Search files

### Storage Dashboard

Monitor:

* Total capacity
* Used capacity
* Available capacity
* Provider status
* Account usage

### Storage Routing

Automatically choose where files are stored based on configurable policies.

Examples:

* Most available space
* Round-robin distribution
* Priority order
* Custom placement rules

### Developer API

Applications can use UniDisk as a storage backend.

Example:

POST /api/files

GET /api/files/{id}

DELETE /api/files/{id}

Applications do not need to know where files are stored.

---

## Deployment

### Docker

```bash
docker run -d \
  --name unidisk \
  -p 8080:8080 \
  -v unidisk-data:/data \
  ghcr.io/unidisk/unidisk:latest
```

### Docker Compose

```yaml
services:
  unidisk:
    image: ghcr.io/unidisk/unidisk:latest
    ports:
      - "8080:8080"
    volumes:
      - unidisk-data:/data

volumes:
  unidisk-data:
```

After deployment:

1. Open the web dashboard.
2. Create an administrator account.
3. Connect storage providers.
4. Start using your unified storage pool.

---

## Roadmap

### MVP

* User authentication
* Google Drive integration
* Multiple Google accounts
* Unified file explorer
* Upload/download
* Storage overview dashboard

### Future

* OneDrive integration
* Dropbox integration
* S3 integration
* Large-file splitting
* Deduplication
* Compression
* WebDAV
* Virtual drive mounting
* Desktop applications

---

## Philosophy

Storage already exists.

UniDisk does not sell storage.

UniDisk helps users unify storage they already own.
