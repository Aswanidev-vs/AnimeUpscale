# AnimeUpscale Web UI

A small browser front-end for `au.exe` in this repo. A tiny Go HTTP server
wraps the CLI: it accepts uploads, streams stdout/stderr over Server-Sent
Events, and serves the resulting image/video for download.

## Run

From the project root (where `au.exe` lives):

```powershell
go build -o web\server.exe .\web
.\web\server.exe
```

Then open <http://127.0.0.1:8080>.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `127.0.0.1:8080` | HTTP listen address |
| `-au`   | auto-detect      | Path to `au.exe` |
| `-workdir` | auto-detect   | Project root (parent of `web/`) |

The server auto-detects the project root by looking for `au.exe` in the
current directory, then in the parent directory.

## Files

- `index.html` — UI shell
- `style.css`  — dark, anime-aesthetic theme
- `app.js`     — drag-drop, form handling, SSE client (vanilla JS)
- `server.go`  — Go HTTP server that wraps `au.exe`

## How it works

1. Browser POSTs the file + form fields to `/api/upscale`.
2. Server saves the upload into a per-job temp dir and spawns
   `au.exe` with the corresponding flags.
3. Browser opens an `EventSource` to `/api/jobs/{id}/events` and
   receives every stdout/stderr line as it streams out, plus a
   final `status` event (`done` / `error`).
4. When the job is done, the browser fetches the result from
   `/api/output/{id}` (also used as the image/video src for
   preview).

Job temp files are stored in `%TEMP%\animeupscale-web\jobs\`
and removed after one hour.

## Limitations

- The server only listens on `127.0.0.1` by default. Don't expose
  it to the public internet — there's no auth and uploads are
  arbitrary files.
- File size cap is 64 MiB on the upload (raise in `server.go` if
  you need larger videos).
- The browser preview only works for the formats the browser can
  decode natively (PNG/JPG/WebP/GIF/MP4/WebM). The download link
  always works regardless.
