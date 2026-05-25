# Image Upscaling Guide

This guide explains how to use this project to upscale images.

## What Works Right Now

The main working native engine in this project is:

- `realsr`

There is also a fallback engine:

- `builtin`

## Project Layout

These paths are important:

- [bin](/e:/AnimeUpscale/bin)
- [models/realsr/models-DF2K_JPEG](/e:/AnimeUpscale/models/realsr/models-DF2K_JPEG)
- [main.go](/e:/AnimeUpscale/main.go)
- [internal](/e:/AnimeUpscale/internal)

Required runtime files for `realsr`:

- [realsr-ncnn-vulkan.exe](/e:/AnimeUpscale/bin/realsr-ncnn-vulkan.exe)
- [vcomp140.dll](/e:/AnimeUpscale/bin/vcomp140.dll)
- [x4.param](/e:/AnimeUpscale/models/realsr/models-DF2K_JPEG/x4.param)
- [x4.bin](/e:/AnimeUpscale/models/realsr/models-DF2K_JPEG/x4.bin)

## Step 1: Open This Folder In Terminal

Work inside:

```powershell
E:\AnimeUpscale
```

## Step 2: Build The CLI

```powershell
go build -o anime-upscaler.exe .
```

This creates:

```powershell
anime-upscaler.exe
```

## Step 3: Check Available Engines

```powershell
.\anime-upscaler.exe -list-engines
```

You should see `realsr` as available.

## Step 4: Upscale An Image With RealSR

Example:

```powershell
.\anime-upscaler.exe -i zegion.jpg -o zegion-realsr-clean-4x.jpg -engine realsr -scale 4
```

What this does:

- `-i zegion.jpg`
  - input image
- `-o zegion-realsr-clean-4x.jpg`
  - output image
- `-engine realsr`
  - uses the working native RealSR backend
- `-scale 4`
  - produces a 4x upscale

## Step 5: Use The Built-In Fallback If Needed

If you want to use the Go fallback instead:

```powershell
.\anime-upscaler.exe -i zegion.jpg -engine builtin -target 4k
```

This preserves aspect ratio and resizes the image so the longest side becomes `3840`.

## Common Commands

Upscale with `realsr`:

```powershell
.\anime-upscaler.exe -i input.jpg -o output.jpg -engine realsr -scale 4
```

Upscale with built-in engine:

```powershell
.\anime-upscaler.exe -i input.jpg -o output.jpg -engine builtin -scale 2
```

Built-in 2K target:

```powershell
.\anime-upscaler.exe -i input.jpg -engine builtin -target 2k
```

Built-in 4K target:

```powershell
.\anime-upscaler.exe -i input.jpg -engine builtin -target 4k
```

## Output Notes

- If you do not pass `-o`, the CLI auto-generates an output filename beside the input.
- `realsr` is currently the best working native option in this project.
- `builtin` works without external inference binaries, but it is not as strong as a dedicated native SR model.

## Troubleshooting

If `realsr` does not appear in `-list-engines`, check that these files still exist:

- [realsr-ncnn-vulkan.exe](/e:/AnimeUpscale/bin/realsr-ncnn-vulkan.exe)
- [vcomp140.dll](/e:/AnimeUpscale/bin/vcomp140.dll)
- [x4.param](/e:/AnimeUpscale/models/realsr/models-DF2K_JPEG/x4.param)
- [x4.bin](/e:/AnimeUpscale/models/realsr/models-DF2K_JPEG/x4.bin)

If Go cannot build, make sure Go is installed and run:

```powershell
go version
```

## Example Output In This Workspace

- [zegion-realsr-clean-4x.jpg](/e:/AnimeUpscale/zegion-realsr-clean-4x.jpg)
