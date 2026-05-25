# Video Upscaling Guide

This guide explains how to upscale videos using this project.

## Prerequisites

### Required System Dependencies

- **ffmpeg** (with ffprobe) — must be installed and available in PATH
- **Go** (only needed if building from source)

Verify ffmpeg is installed:

```powershell
ffmpeg -version
ffprobe -version
```

If missing, download ffmpeg from https://ffmpeg.org/download.html and add it to your PATH.

### Project Runtime Files

These are already bundled in the project workspace:

- `bin/realsr-ncnn-vulkan.exe` — RealSR native inference binary
- `bin/vcomp140.dll` — Visual C++ runtime for the inference binary
- `models/realsr/models-DF2K_JPEG/` — RealSR model files (`x4.param`, `x4.bin`)
- `animeupscale.exe` — pre-built CLI (or build your own)

## Step 1: Open This Folder In Terminal

Work inside:

```powershell
E:\AnimeUpscale
```

## Step 2: Build The CLI (if not already built)

```powershell
go build -o anime-upscaler.exe .
```

This creates `anime-upscaler.exe` (or use the pre-built `animeupscale.exe`).

## Step 3: Verify The Engines Are Detected

```powershell
.\animeupscale.exe -list-engines
```

You should see `realsr` listed as **available**. The video pipeline requires `realsr` (or another native backend) — it does not support the `builtin` engine for video mode.

## Step 4: Upscale A Video

### Basic 4K Upscale (Recommended)

```powershell
.\animeupscale.exe -video -i kaisaki.mp4 -target 4k -engine realsr
```

What this does:
- `-video` — treat input as video (can be omitted for `.mp4`/`.mkv`/`.mov`/`.avi`/`.webm` files; auto-detected by extension)
- `-i kaisaki.mp4` — input video
- `-target 4k` — final output resolution; longest side becomes 3840px, aspect ratio preserved
- `-engine realsr` — use the RealSR native engine (this is the default for video mode)

The pipeline does the following:
1. **probe** — reads video metadata (resolution, FPS, audio presence) with ffprobe
2. **extract** — extracts all frames as PNG images into `temp/<video-name>/frames/`
3. **upscale** — runs RealSR on every frame, saving to `temp/<video-name>/upscaled/`
4. **encode** — re-encodes the upscaled frames into a video at the target resolution using ffmpeg
5. **mux audio** — copies the original audio track into the final output (if present)

On completion, the temp directory is automatically deleted.

### 2K Upscale

```powershell
.\animeupscale.exe -video -i input.mp4 -target 2k -engine realsr
```

Longest side becomes 2048px, aspect ratio preserved.

## Advanced Options

### Frame Rate Override

```powershell
.\animeupscale.exe -video -i input.mp4 -target 4k -fps 30
```

By default, the output video uses the source video's frame rate. Use `-fps` to override (e.g. `24`, `30`, `60`).

### Keep Temporary Files

```powershell
.\animeupscale.exe -video -i input.mp4 -target 4k -keep-temp
```

This retains `temp/<video-name>/` after completion, useful for:
- Debugging frame extraction or upscaling issues
- Re-encoding with custom ffmpeg settings manually
- Inspecting individual frames

### Custom Video Codec

```powershell
.\animeupscale.exe -video -i input.mp4 -target 4k -video-codec libx265
```

Available codecs depend on your ffmpeg build. Common options:
- `libx264` (default) — H.264, widely compatible
- `libx265` — HEVC/H.265, better compression
- `libvpx-vp9` — VP9, open codec

### Custom Output Path

```powershell
.\animeupscale.exe -video -i input.mp4 -o D:\output\result.mp4 -target 4k
```

Without `-o`, the output is auto-named as `<input-name>-4k.mp4` (or `<input-name>-2k.mp4`) in the same directory as the input.

### Upscale Without Explicit Video Flag

Video files are auto-detected by extension:

```powershell
.\animeupscale.exe -i input.mp4 -target 4k
```

This works for `.mp4`, `.mkv`, `.mov`, `.avi`, and `.webm` files.

## How The Pipeline Works

```
Input Video
    │
    ▼
[ffprobe] ──► metadata (resolution, FPS, audio)
    │
    ▼
[ffmpeg] ──► extract frames (PNG)
    │
    ▼
[RealSR per frame] ──► upscaled frames (PNG)
    │
    ▼
[ffmpeg] ──► scale to target (2k/4k) + re-encode + mux audio
    │
    ▼
Output Video
```

### Temp Directory Layout

```
temp/
└── <video-name>/
    ├── frames/        # extracted source frames (frame_000001.png, ...)
    └── upscaled/      # upscaled frames   (frame_000001.png, ...)
```

## Examples With The Pre-Built Binary

If you're using `animeupscale.exe` instead of building `anime-upscaler.exe`:

```powershell
.\animeupscale.exe -video -i kaisaki.mp4 -target 4k -engine realsr
```

```powershell
.\animeupscale.exe -video -i kaisaki.mp4 -target 2k -engine realsr
```

```powershell
.\animeupscale.exe -i kaisaki.mp4 -target 4k
```

## Output Notes

- Output format is always **MP4** (H.264 by default) with the original audio track preserved.
- Subtitles and additional streams (e.g. multiple audio tracks) are **not preserved** in v1.
- If the input video has no audio track, the output will also be silent (audio muxing is skipped).
- The final video is re-encoded entirely (not stream-copied), which preserves maximum quality from the upscaled frames.

## Troubleshooting

### "ffmpeg is required but was not found in PATH"

Install ffmpeg and ensure both `ffmpeg.exe` and `ffprobe.exe` are accessible from your terminal. Restart the terminal after installation.

### "video mode does not support builtin engine"

The video pipeline requires a native inference backend (RealSR, Real-ESRGAN, Waifu2x, etc.). Run `-list-engines` to check which native engines are available.

### "realsr is unavailable for video mode"

Check that these files exist:

- `bin/realsr-ncnn-vulkan.exe`
- `bin/vcomp140.dll`
- `models/realsr/models-DF2K_JPEG/x4.param`
- `models/realsr/models-DF2K_JPEG/x4.bin`

If they are missing, restore them from the repository or re-download the RealSR ncnn Vulkan package.

### "video mode requires -target 2k or -target 4k"

The video pipeline always needs a target resolution (`-target 2k` or `-target 4k`). This is intentional — the pipeline scales upscaled frames to a fixed output resolution.

### Upscale Is Too Slow

Video upscaling processes every frame individually through the neural network. For long videos, this can take significant time. Consider:
- Processing a shorter clip first to verify settings
- Using a GPU with Vulkan support for faster inference
- Reducing the output FPS if appropriate

### Temp Files Consuming Disk Space

If a pipeline fails, temp files are kept automatically. Clean them up manually:

```powershell
Remove-Item -Recurse -Force temp\