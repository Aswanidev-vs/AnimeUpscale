# Anime Upscaler CLI

A Go command-line tool for upscaling anime and illustration images.

This project currently has:

- a working `realsr` backend using a project-level `bin` + `models` layout
- a built-in pure Go fallback for simple upscale and enhancement
- optional wrappers for `anime4kcpp`, `waifu2x`, `realcugan`, and `realesrgan` if you add those tools separately

## Current Status

The main working native backend in this workspace is:

- `realsr`

It uses:

- [bin](/e:/AnimeUpscale/bin)
- [models/realsr](/e:/AnimeUpscale/models/realsr)

The project is designed so Go stays the main CLI, while native upscalers do the heavy image inference work.

## Build

```powershell
go build -o anime-upscaler.exe .
```

## Quick Start

List detected engines:

```powershell
go run . -list-engines
```

Upscale an image with the working `realsr` backend:

```powershell
go run . -i zegion.jpg -o zegion-realsr-clean-4x.jpg -engine realsr -scale 4
```

Use the built-in fallback:

```powershell
go run . -i zegion.jpg -engine builtin -target 4k
```

## Supported Engines

- `realsr`
  - Working in this workspace.
  - Uses [realsr-ncnn-vulkan.exe](/e:/AnimeUpscale/bin/realsr-ncnn-vulkan.exe).
  - Uses NCNN model files under [models/realsr](/e:/AnimeUpscale/models/realsr).
- `builtin`
  - Pure Go fallback.
  - Works without external binaries.
- `anime4kcpp`
  - Wrapper only.
  - Requires `ac_cli` to be installed separately.
- `waifu2x`
  - Wrapper only.
  - Requires `waifu2x-ncnn-vulkan` to be installed separately.
- `realcugan`
  - Wrapper only.
  - Requires `realcugan-ncnn-vulkan` to be installed separately.
- `realesrgan`
  - Wrapper only.
  - Requires `realesrgan-ncnn-vulkan` plus matching Real-ESRGAN NCNN model files.

## CLI Usage

```powershell
anime-upscaler.exe -i input.png -o output.png -engine auto -scale 2
anime-upscaler.exe -i input.png -o output.png -engine realsr -scale 4
anime-upscaler.exe -i input.png -engine builtin -target 4k
anime-upscaler.exe -i input.png -o output.png -engine realesrgan -scale 4 -model-name realesr-animevideov3
anime-upscaler.exe -list-engines
```

## Target Presets

The built-in engine supports:

- `-target 2k`
  - sets the longest side to `2048`
- `-target 4k`
  - sets the longest side to `3840`

Aspect ratio is preserved automatically.

## Output Behavior

- If `-o` is omitted, the tool auto-generates an output filename beside the input.
- If `-format` is omitted, the tool preserves the input/output extension when possible.

## Notes About `realsr`

This workspace now uses a cleaner project-level layout:

- [bin](/e:/AnimeUpscale/bin)
- [models/realsr/models-DF2K_JPEG](/e:/AnimeUpscale/models/realsr/models-DF2K_JPEG)

The native binary still expects model directory names compatible with its own logic, so the Go code points it to the compatible nested model folder automatically.

## Project Structure

- [main.go](/e:/AnimeUpscale/main.go)
  - entrypoint
- [internal/cli](/e:/AnimeUpscale/internal/cli)
  - CLI parsing and request building
- [internal/upscale](/e:/AnimeUpscale/internal/upscale)
  - engine selection and backend integration
- [internal/nativeexec](/e:/AnimeUpscale/internal/nativeexec)
  - native execution bridge, including the Windows `cgo` runner

## Example Result

Generated from the current workspace:

- [zegion-realsr-clean-4x.jpg](zegion_upscale-4k.png)


## References

- RealSR NCNN Vulkan: https://github.com/nihui/realsr-ncnn-vulkan
- Anime4KCPP: https://github.com/TianZerL/Anime4KCPP
- waifu2x NCNN Vulkan: https://github.com/nihui/waifu2x-ncnn-vulkan
- RealCUGAN NCNN Vulkan: https://github.com/nihui/realcugan-ncnn-vulkan
- Real-ESRGAN NCNN Vulkan: https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan
