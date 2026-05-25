# Video Upscaling Implementation Plan

## Summary

Add a first-class video mode to the Go CLI that upscales a video end to end using `ffmpeg` plus the existing `realsr` image backend. The v1 flow will extract frames, upscale them, optionally resize to a final `2k` or `4k` target while preserving aspect ratio, then re-encode the video and preserve the original audio track.

Defaults chosen:
- single-command workflow
- target-based output (`2k` / `4k`) as the primary UX
- preserve audio, ignore subtitles in v1
- `realsr` as the default video frame engine
- intermediate work done in a temp job directory, cleaned up on success

## Key Changes

### CLI and public behavior
- Add a video entry path to the existing CLI rather than a separate binary.
- Support video inputs by file extension and/or an explicit mode flag such as `-video`.
- Add video-oriented flags:
  - `-target 2k|4k` for final output resolution
  - `-fps` optional override; otherwise preserve source fps
  - `-keep-temp` optional debug flag to retain extracted/upscaled frames
  - `-video-codec` optional advanced override, defaulting to a sane H.264 output
- Keep the current image flow unchanged.
- If `-engine auto` is used for video, prefer `realsr`, then fail clearly if no compatible native backend is available.

### Video pipeline
- Add a new video orchestration layer that:
  - probes the input video with `ffprobe` or `ffmpeg` metadata
  - extracts frames to a temp directory
  - upscales frames through the existing engine abstraction, using `realsr` per frame
  - applies final aspect-ratio-preserving resize to the requested `2k` or `4k` target after model upscale
  - re-encodes the frames into a video
  - copies the original audio track into the final output when present
- Use a deterministic temp layout such as:
  - `temp/<job-id>/frames`
  - `temp/<job-id>/upscaled`
  - `temp/<job-id>/final`
- Fail fast on missing `ffmpeg` or missing engine binaries with direct actionable errors.

### Engine and processing integration
- Reuse `internal/upscale` for frame upscaling rather than introducing a separate code path.
- Add a thin video package or subsystem that calls the current image engine for each extracted frame.
- For `2k` and `4k` video targets:
  - keep aspect ratio
  - map longest side to `2048` or `3840`
  - use `ffmpeg` scaling for final video-sized normalization after SR output
- Preserve image format consistency for intermediate frames, using PNG for loss-minimized processing in v1.

### Errors, progress, and cleanup
- Print high-level stage progress:
  - probe
  - extract
  - upscale
  - encode
  - mux audio
- Surface native `realsr` and `ffmpeg` stderr cleanly.
- Delete temp artifacts on success by default.
- Keep temp artifacts on failure, and optionally on success with `-keep-temp`.

## Test Plan

### Functional scenarios
- Upscale a short MP4 with audio to `4k`; verify output video exists, plays, and has audio.
- Upscale a short MP4 with audio to `2k`; verify aspect ratio is preserved.
- Upscale a silent video; verify output succeeds without audio.
- Run with `-engine realsr`; verify frame upscale path uses the current native backend.
- Run with `-engine auto`; verify it selects `realsr` when available.

### Failure scenarios
- Missing `ffmpeg`; verify clear error before work starts.
- Missing `realsr` runtime files; verify clear error before frame extraction begins.
- Invalid input path or unsupported container; verify clean failure.
- Corrupt or interrupted frame extraction; verify partial temp files remain available for debugging.

### Regression checks
- Existing image commands still behave exactly as before.
- `-target 2k|4k` for images remains limited to the built-in image path unless explicitly expanded later.
- `-list-engines` behavior remains stable for image backends.

## Assumptions and defaults

- v1 supports local file input and file output only, not streaming URLs.
- v1 targets one video at a time, not directory-wide batch video processing.
- v1 preserves the original audio stream when present, but does not preserve subtitles or additional streams.
- v1 re-encodes video rather than trying to stream-copy the video track.
- v1 uses `realsr` for video because it is the working native backend already present in this project.
- v1 does not implement scene-aware filtering, deinterlacing, HDR handling, or face restoration.
