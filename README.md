# reMarkable QMD Hasher

A web application for hashing QMD (QML Diff) files using GCD (Greatest Common Divisor) hashtabs generated from multiple reMarkable device variants.

## Overview

This tool allows you to:
1. Upload unhashed QMD files
2. Select a target reMarkable OS version
3. Download the files hashed with a GCD hashtab that works across all device variants (rm1, rm2, rmpp, rmppm)

## Requirements

- Go 1.23+
- Node.js 20+
- Rust/Cargo (for building qmldiff)
- Device-specific hashtables in the `hashtables/` directory

## Development Setup

1. Clone the repository:
```bash
git clone https://github.com/rmitchellscott/rm-qmd-hasher
cd rm-qmd-hasher
```

2. Build the qmldiff CLI:
```bash
git clone --branch qmdverify https://github.com/rmitchellscott/qmldiff
cd qmldiff
cargo build --release --bin qmldiff
cp target/release/qmldiff ../
cd ..
```

3. Install frontend dependencies:
```bash
cd ui
npm install
npm run build
cd ..
```

4. Add hashtables:
```bash
mkdir -p hashtables
# Copy device-specific hashtables here, e.g.:
# hashtables/3.24.0.149-rm1
# hashtables/3.24.0.149-rm2
# hashtables/3.24.0.149-rmpp
# hashtables/3.24.0.149-rmppm
```

5. Run the server:
```bash
go run .
```

The server will start on http://localhost:8080

## Docker

Build and run with Docker:

```bash
docker build -t rm-qmd-hasher .
docker run -p 8080:8080 -v ./hashtables:/app/hashtables:ro rm-qmd-hasher
```

Or use docker-compose:

```bash
docker-compose up -d
```

## API

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/versions` | List available OS versions |
| POST | `/api/hash` | Upload QMD files for hashing |
| GET | `/api/results/{jobId}` | Get job status and results |
| GET | `/api/download/{jobId}` | Download hashed files |
| WS | `/api/status/ws/{jobId}` | WebSocket for real-time progress |
| GET | `/api/version` | Application version info |

### GET /api/versions

Returns available OS versions and their device variants.

**Response:**
```json
{
  "count": 1,
  "versions": [
    {
      "version": "3.25.0.140",
      "devices": ["rm1", "rm2", "rmpp", "rmppm"],
      "deviceCount": 4
    }
  ]
}
```

### POST /api/hash

Upload QMD files for hashing with a GCD hashtab.

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | Yes | Target OS version (e.g., `3.25.0.140`) |
| `files` | file(s) | Yes | One or more QMD files to hash |
| `paths` | string(s) | Yes | Corresponding path for each file (preserves directory structure in ZIP output) |

**Example:**
```bash
curl -X POST http://localhost:8080/api/hash \
  -F "version=3.25.0.140" \
  -F "files=@myfile.qmd" \
  -F "paths=myfile.qmd"
```

**Multiple files:**
```bash
curl -X POST http://localhost:8080/api/hash \
  -F "version=3.25.0.140" \
  -F "files=@file1.qmd" \
  -F "paths=folder/file1.qmd" \
  -F "files=@file2.qmd" \
  -F "paths=folder/file2.qmd"
```

**Response:**
```json
{
  "jobId": "eda763c6-9ecf-4b6e-ab8a-e3c55287c86c"
}
```

### GET /api/results/{jobId}

Get the status and results of a hashing job.

**Response (processing):**
```json
{
  "status": "processing",
  "message": "Hashing files...",
  "progress": 50,
  "operation": "hashing",
  "fileCount": 2
}
```

**Response (complete):**
```json
{
  "status": "success",
  "message": "Hashed 2 file(s)",
  "fileCount": 2,
  "files": [
    {
      "name": "file1.qmd",
      "path": "folder/file1.qmd",
      "status": "success"
    },
    {
      "name": "file2.qmd",
      "path": "folder/file2.qmd",
      "status": "success"
    }
  ]
}
```

**Response (error):**
```json
{
  "status": "error",
  "message": "qmldiff hash-diffs failed: exit status 1"
}
```

### GET /api/download/{jobId}

Download hashed files. Returns the file directly for single-file jobs, or a ZIP archive for multi-file jobs.

**Response Headers:**
- Single file: `Content-Disposition: attachment; filename="filename.qmd"`
- Multiple files: `Content-Disposition: attachment; filename="hashed-files.zip"`

**Example:**
```bash
# Download single file
curl -o hashed.qmd http://localhost:8080/api/download/{jobId}

# Download ZIP (multiple files)
curl -o hashed.zip http://localhost:8080/api/download/{jobId}
```

### WS /api/status/ws/{jobId}

WebSocket endpoint for real-time job progress updates. Messages are JSON with the same format as `/api/results/{jobId}`.

**Example (JavaScript):**
```javascript
const ws = new WebSocket('ws://localhost:8080/api/status/ws/' + jobId);
ws.onmessage = (event) => {
  const status = JSON.parse(event.data);
  console.log(status.progress + '%', status.message);
};
```

### GET /api/version

Returns application version information.

**Response:**
```json
{
  "version": "1.0.0",
  "commit": "abc1234",
  "buildTime": "2024-01-15T10:30:00Z"
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | Server port |
| HASHTAB_DIR | ./hashtables | Directory containing device hashtables |
| GCD_HASHTAB_DIR | ./gcd-hashtabs | Directory for generated GCD hashtabs |
| QMLDIFF_BINARY | ./qmldiff | Path to qmldiff CLI binary |

## License

MIT
