# Anime Upscaler CLI

A Go command-line tool for upscaling anime and illustration images.

This project currently has:

- a working `realesrgan` backend using project-level `bin` + `models` layout (recommended for anime)
- a working `realsr` backend using a project-level `bin` + `models` layout
- a built-in pure Go fallback for simple upscale and enhancement
- optional wrappers for `anime4kcpp`, `waifu2x`, and `realcugan` if you add those tools separately

## Current Status

The working native backends in this workspace are:

### Real-ESRGAN (Recommended)

- Binary: [bin/realesrgan-ncnn-vulkan.exe](/e:/AnimeUpscale/bin/realesrgan-ncnn-vulkan.exe)
- Models: [models/](/e:/AnimeUpscale/models/) (includes `realesr-animevideov3-x2/3/4`, `realesrgan-x4plus`, `realesrgan-x4plus-anime`)
- Default engine for video mode
- Optimized anime model: `realesr-animevideov3-x4`

### RealSR

- Binary: [bin/realsr-ncnn-vulkan.exe](/e:/AnimeUpscale/bin/realsr-ncnn-vulkan.exe)
- Models: [models/realsr/](/e:/AnimeUpscale/models/realsr/)

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

Upscale an image with Real-ESRGAN (recommended for anime):

```powershell
go run . -i zegion.jpg -o o.png -engine realesrgan -scale 4 -model-name realesr-animevideov3-x4
```

Use a different Real-ESRGAN model (photorealistic):

```powershell
go run . -i zegion.jpg -o o.png -engine realesrgan -scale 4 -model-name realesrgan-x4plus
```

Use the built-in fallback:

```powershell
go run . -i zegion.jpg -engine builtin -target 4k
```

Upscale with `realsr`:

```powershell
go run . -i zegion.jpg -o zegion-realsr-clean-4x.jpg -engine realsr -scale 4
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
  - Working in this workspace.
  - Uses [realesrgan-ncnn-vulkan.exe](/e:/AnimeUpscale/bin/realesrgan-ncnn-vulkan.exe).
  - Bundled anime models: `realesr-animevideov3-x2`, `realesr-animevideov3-x3`, `realesr-animevideov3-x4`.
  - Also bundles `realesrgan-x4plus` and `realesrgan-x4plus-anime`.
  - Default engine for video mode; recommended for anime content.
  - Use `-model-name` to select a model (default: `realesr-animevideov3`).
  - Model path auto-detected from [models/](/e:/AnimeUpscale/models/).

## CLI Usage

```powershell
anime-upscaler.exe -i input.png -o output.png -engine auto -scale 2
anime-upscaler.exe -i input.png -o output.png -engine realsr -scale 4
anime-upscaler.exe -i input.png -engine builtin -target 4k
anime-upscaler.exe -i input.png -o output.png -engine realesrgan -scale 4 -model-name realesr-animevideov3
anime-upscaler.exe -list-engines
```

## Video Benchmark Mode

When upscaling videos (`.mp4/.mkv/.mov/.avi/.webm`) you can enable per-stage timing with:

```powershell
anime-upscaler.exe -i input.mp4 -o output.mp4 -engine realesrgan -target 4k -scale 4 -benchmark
```

Notes:
- Timing is printed to the terminal for these stages:
  - `probe`
  - `extract`
  - `upscale`
  - `encode/mux` (and audio mux is included when present)
- A JSON file is written to the project root: `bench.json`
- Video concurrency is controlled by `-video-workers` (default: `1`):

```powershell
anime-upscaler.exe -video -i input.mp4 -o output.mp4 -engine realesrgan -target 4k -scale 4 -video-workers 4 -benchmark
```
anime-upscaler.exe -video -i input.mp4 -o output.mp4 -engine realesrgan -target 4k -scale 4 -video-workers 4 -benchmark
anime-upscaler.exe -i input.mp4 -o output.mp4 -engine realesrgan -target 4k -scale 4 -benchmark
anime-upscaler.exe -list-engines

## Real-ESRGAN Models

The following models are bundled under [models/](/e:/AnimeUpscale/models/):

| Model Name                  | Scale | Type                     | Command |
|-----------------------------|-------|--------------------------|---------|
| `realesr-animevideov3`      | 2     | Anime (default)          | `-scale 2 -model-name realesr-animevideov3` |
| `realesr-animevideov3-x3`   | 3     | Anime                    | `-scale 3 -model-name realesr-animevideov3-x3` |
| `realesr-animevideov3-x4`   | 4     | Anime (recommended)      | `-scale 4 -model-name realesr-animevideov3-x4` |
| `realesrgan-x4plus`         | 4     | Photorealistic           | `-scale 4 -model-name realesrgan-x4plus` |
| `realesrgan-x4plus-anime`   | 4     | Anime photorealistic 6B  | `-scale 4 -model-name realesrgan-x4plus-anime` |

**Examples:**

```powershell
# Anime image (default model, 2x upscale)
.\us.exe -i mpeak.png -o mai.png -engine realesrgan -scale 2 -model-name realesr-animevideov3

# Anime image (4x upscale)
.\us.exe -i mpeak.png -o mai.png -engine realesrgan -scale 4 -model-name realesr-animevideov3-x4

# Photorealistic image (4x upscale)
.\us.exe -i zegion.jpg -o o.png -engine realesrgan -scale 4 -model-name realesrgan-x4plus

# Anime photorealistic (4x upscale, 6B variant)
.\us.exe -i zegion.jpg -o o.png -engine realesrgan -scale 4 -model-name realesrgan-x4plus-anime

# Video (4x upscale → 2K output)
.\us.exe -i sky.mp4 -o sky-2k.mp4 -engine realesrgan -scale 4 -target 2k -model-name realesr-animevideov3-x4

# Video (4x upscale → 4K output)
.\us.exe -i sky.mp4 -o sky-4k.mp4 -engine realesrgan -scale 4 -target 4k -model-name realesr-animevideov3-x4
```

> **Note:** The model name is case-sensitive and must match exactly. The default model if `-model-name` is omitted is `realesr-animevideov3` (the `-x2` variant).

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
