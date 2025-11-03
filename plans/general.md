# ImageServer – Comprehensive Overview

## Purpose
- Provide a lightweight, self-hosted image server for hosting and serving images.
- Offer REST endpoints for directory listing, file deletion, directory creation, and image upload.
- Support on-demand image variant generation (e.g., preview) and basic format conversion.
- Keep security simple with Basic Auth on management endpoints and path-safety checks for public image serving.

## Tech Stack
- Language: Go
- Web framework: `github.com/gin-gonic/gin`
- Imaging libraries:
  - Standard library `image`, `image/png`, `image/jpeg`
  - `golang.org/x/image/draw` for high-quality scaling (CatmullRom)
  - `golang.org/x/image/webp` for decoding WebP (encoding commented out)

## Project Structure
```
ImageServer/
├── config/        # Configuration loading (env-based)
├── handlers/      # HTTP handlers (REST API and public image serving)
├── middleware/    # BasicAuth and CORS handlers
├── models/        # Data structures (FileInfo, type lists)
├── utils/         # Image helpers: find, load, save, scale, variants
├── main.go        # App bootstrap and route wiring
├── Dockerfile     # Containerization (not detailed here)
├── README.md      # Quickstart
├── go.mod, go.sum # Dependencies
└── plans/         # Documentation (this file)
```

## Configuration
- Source: `config/config.go`
- Type: `Config` with fields:
  - `Path`: data root for images and folders (default `./data`)
  - `Port`: HTTP port (default `5000`)
  - `Username`, `Password`: Basic Auth credentials
  - `Domain`: base URL used to return file URLs in API responses
- Environment variables:
  - `DATA_PATH`, `PORT`, `SERVER_USERNAME`, `SERVER_PASSWORD`, `IMAGE_SERVER_DOMAIN`
- Loading strategy: `getEnv(key, default)` reads env or falls back to defaults.

## Startup Flow (main.go)
- Set Gin to release mode.
- Load config and ensure the data directory exists (`os.MkdirAll`).
- Create Gin router and attach middleware:
  - `CORS()` allowing `*` origins, methods `GET, POST, PUT, DELETE, OPTIONS`, headers `Authorization, Content-Type`, and handling OPTIONS.
- Initialize handlers:
  - `ImageHandler` for public image serving
  - `APIHandler` for protected management endpoints
- Define routes:
  - Group `/api/v1` with `BasicAuth(username, password)` for protected endpoints.
  - Fallback `NoRoute`:
    - For `GET`, forward to `ImageHandler.ServeImage` (public image serving)
    - For non-GET, return `404` JSON
- Log startup info and listen on `cfg.Port`.

## Security
- Basic Auth: `middleware.BasicAuth` wraps `gin.BasicAuth(gin.Accounts{username: password})` and protects all `/api/v1` endpoints.
- CORS: permissive; suitable for controlled environments. Adjust for production if needed.
- Path safety in public serving (`handlers/image.go`):
  - Clean and normalize `filepath`.
  - Reject absolute paths and traversal sequences (`..`).
  - Ensure resolved path remains within the configured base directory.

## Public Image Serving
- Entry: `ImageHandler.ServeImage(c)` via `NoRoute` for `GET` requests.
- Behavior:
  - Query `variant` optional; formats inferred from path extension.
  - Cache headers: `Cache-Control: public, max-age=31536000` (1 year).
  - Supported types: `png`, `jpg`, `jpeg`, `gif`, `webp`, `svg` (see `models.SupportedTypes`).
  - Convertible types: `png`, `jpg`, `jpeg` (see `models.ConverableTypes`).
  - Fast-path:
    - If format is empty or `png` and no `variant`: serve the base file (stored without extension after conversion).
    - If format is not convertible: serve file with extension directly.
    - If `variant` is empty and the exact file exists: serve it.
  - Variant handling:
    - Build `variantPath = <file>.<variant>.<format>`.
    - If exists, serve directly.
    - Otherwise, generate via `utils.ReadImage(filePathNoExt, variant, format, variantPath)`:
      - `FindImage` falls back among `.png`, `.jpg`, `.webp`, `.jpeg`.
      - `loadImage` decodes into `image.Image`.
      - `ApplyVariant("preview")` scales longest side to 256 using CatmullRom.
      - `save(variantPath, img, ext)` writes PNG or JPEG (WebP encode commented).
    - Respond `201 Created` and serve the generated variant file.

## REST API (Protected, Basic Auth)
- Base: `/api/v1`
- Endpoints (`handlers/api.go`):
  - `GET /files/*path` — List directory contents
    - Query: `size` (default 10), `page` (default 0)
    - Returns: JSON array of `models.FileInfo` (name, path, size, modTime, isDir)
    - Skips dotfile entries via `utils.ContainsDotFile`
  - `POST /directories/*path` — Create directory
    - Creates nested directories under `Config.Path`.
    - Returns `201 Created` with message.
  - `POST /images` — Upload image
    - Form fields: `folder`, `id`, `format`, and file field `file`.
    - Ensures folder exists; reads file bytes.
    - Behavior:
      - If requested `format` is NOT convertible (`!ConverableTypes.Has(format)`):
        - Save as `<id>.<format>` in the target folder.
      - If requested `format` is convertible:
        - If `format == "png"`: save raw bytes.
        - Else: decode image and re-encode as PNG, then save.
      - Respond with `201 Created` and a URL composed from `Config.Domain` + `/<folder>/<id>.<format>`.
    - Notes: underlying saved filename for converted PNG uses `<id>` without extension; the public URL includes `.<format>` as requested.
  - `DELETE /files/*path` — Delete file or directory
    - Attempts to remove files with the same basename (strip extension) first, then deletes the exact file or directory.
    - Returns `200 OK` with confirmation message.

## Models
- `models.FileInfo`: struct returned by list endpoint.
- `models.ExtSlice`: helper to track supported and convertible formats.
- `models.SupportedTypes`: `jpg`, `png`, `gif`, `webp`, `jpeg`, `svg`.
- `models.ConverableTypes`: `jpg`, `png`, `jpeg`.

## Utilities (`utils/image.go`)
- `ContainsDotFile(path)`: detects dot-prefixed components, used to filter listings.
- `FindImage(base)`: attempts to open the file by trying common extensions.
- `loadImage(path)`: open + `image.Decode`.
- `save(path, img, ext)`: save as PNG or JPEG; WebP encode commented out.
- `Scale(img, size)`: keep aspect ratio, scale longest side to `size` using CatmullRom.
- `ApplyVariant(img, variant)`: supports `preview` variant; identity otherwise.
- `Preview(img)`: convenience wrapper over `Scale(..., 256)`.
- `FixAllFiles(cfg)`: walk the data directory, decode existing images by extension, and write a PNG alongside without extension. Useful for normalizing storage; review behavior before running in production.

## Error Handling & Logging
- Uses `log.Printf` and `log.Fatalf` in startup and utils.
- Handlers return JSON with `error` messages and appropriate HTTP status codes.

## Deployment Notes
- A `Dockerfile` is present for container builds (multi-stage); configure env vars appropriately.
- For production, place behind a reverse proxy (TLS termination, rate limiting, auth) and refine CORS.

## Usage Examples
- Set environment:
```
export DATA_PATH=/var/lib/images
export PORT=5000
export SERVER_USERNAME=myuser
export SERVER_PASSWORD=secret
export IMAGE_SERVER_DOMAIN=https://images.example.com
```
- Upload image:
```
curl -u myuser:secret -F folder=avatars -F id=user123 -F format=png -F file=@avatar.jpg \
  https://images.example.com/api/v1/images
```
- List directory:
```
curl -u myuser:secret "https://images.example.com/api/v1/files/avatars?page=0&size=20"
```
- Serve image (public):
```
# Original
GET https://images.example.com/avatars/user123.png

# Variant (preview)
GET https://images.example.com/avatars/user123.png?variant=preview
```

## Known Limitations / Considerations
- Basic Auth only; consider stronger auth or IP allowlists for admin APIs.
- CORS is permissive; lock down origins in production.
- WebP encode is not enabled; add when needed.
- Deletion behavior removes files and directories; ensure correct path inputs to avoid unintended removal.
- Converted PNGs for uploads are stored without extension; ensure serving logic or storage strategy meets your needs.