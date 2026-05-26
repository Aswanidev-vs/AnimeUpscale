# TODO - Video progress bar + parallel frame upscaling

- [ ] Update `internal/cli/cli.go` to add `-video-workers` flag and pass it to `video.Config`.
- [ ] Update `internal/video/video.go`:
  - [ ] Add `Workers` to `video.Config`.
  - [ ] Implement a simple text progress bar during the upscale stage.
  - [ ] Parallelize frame upscaling using a worker pool (bounded by `Workers`).
  - [ ] Preserve deterministic error handling: if any frame fails, return the error.
- [ ] Run `go test ./...` (if applicable) and `go build ./...` to ensure compilation.
