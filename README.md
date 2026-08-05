# Anime Upscaler 

A high-performance Go command-line tool for upscaling anime, illustrations, and videos provide both **cli** and  **browser-based GUI**.

This project packages:
- A working **Real-ESRGAN** native backend using a project-level `bin/` + `models/` layout (highly recommended for anime).
- A working **RealSR** native backend using a project-level `bin/` + `models/` layout.
- A built-in, zero-dependency, pure Go fallback for quick upscale and enhancement.
- Wrappers for `anime4kcpp`, `waifu2x`, and `realcugan` (requires installing their CLI tools separately).
- A **Web UI** for drag-and-drop upscaling directly from your browser — no terminal needed.

---

## Prerequisites

Before using Anime Upscaler, make sure the following are installed on your system.

### 1. Go (Required for building)

**Check:**
```powershell
go version
# Expected: go version go1.21+ windows/amd64
```

**Install:** Download from [https://go.dev/dl/](https://go.dev/dl/) and run the installer. Restart your terminal after install.

---

### 2. FFmpeg & FFprobe (Required for video mode)

These are needed **only** for video upscaling (`-video` flag). Image-only upscaling does not need them.

**Check:**
```powershell
ffmpeg -version
ffprobe -version
# Both should print version info if installed
```

**Install (Windows — pick one):**

**Option A — winget (recommended):**
```powershell
winget install --id Gyan.FFmpeg -e --source winget
```

**Option B — scoop:**
```powershell
scoop install ffmpeg
```

**Option C — Manual:**
1. Download from [https://www.gyan.dev/ffmpeg/builds/](https://www.gyan.dev/ffmpeg/builds/) (get the `ffmpeg-release-essentials.zip`)
2. Extract to a folder (e.g. `C:\ffmpeg`)
3. Add `C:\ffmpeg\bin` to your system PATH:
   ```powershell
   # Verify after adding to PATH:
   ffmpeg -version
   ```

> **Note:** The CLI checks for ffmpeg/ffprobe in your global PATH first. If not found, it falls back to checking `./bin/` and current directory.

---

### 3. Vulkan-Compatible GPU (Required for native engines)

Real-ESRGAN, RealSR, waifu2x, and realcugan all use Vulkan GPU acceleration.

**Check:**
```powershell
# List detected GPUs when running any upscale:
au.exe -list-engines
# Should show available engines with GPU info
```

**Requirements:**
- NVIDIA: Install latest [GeForce drivers](https://www.nvidia.com/download/index.aspx)
- AMD: Install latest [Adrenalin drivers](https://www.amd.com/en/support)
- Intel: Install latest [Arc/UHD drivers](https://www.intel.com/content/www/us/en/download-center/home.html)

> **Tip:** If you have dual GPUs (e.g. integrated AMD + dedicated NVIDIA), use `-gpu 1` to target the dedicated GPU and avoid integrated GPU VRAM crashes.

---

### 4. Upscaling Engine Binaries (At least one required)

The CLI auto-detects engines from `./bin/`, `./models/`, or your PATH.

| Engine | Binary | Where to get |
|--------|--------|-------------|
| **Real-ESRGAN** (recommended) | `realesrgan-ncnn-vulkan.exe` | [GitHub Releases](https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan/releases) |
| **RealSR** | `realsr-ncnn-vulkan.exe` | [GitHub Releases](https://github.com/nihui/realsr-ncnn-vulkan/releases) |
| **waifu2x** | `waifu2x-ncnn-vulkan.exe` | [GitHub Releases](https://github.com/nihui/waifu2x-ncnn-vulkan/releases) |
| **realcugan** | `realcugan-ncnn-vulkan.exe` | [GitHub Releases](https://github.com/nihui/realcugan-ncnn-vulkan/releases) |
| **Anime4KCPP** | `ac_cli.exe` | [GitHub Releases](https://github.com/TianZerL/Anime4KCPP/releases) |
| **Built-in** | *None needed* | Pure Go — always available |

**Quick setup:** Place the engine `.exe` and its `models/` folder in `./bin/` next to `au.exe`, or add them to your system PATH.

**Verify engines detected:**
```powershell
au.exe -list-engines
# Shows all detected engines and their paths
```

---

## Build / Install

### Recommended Install
Compile and install the `animeupscale` CLI directly to your `%GOPATH%/bin`:
```powershell
go install github.com/Aswanidev-vs/animeupscale@latest
```

Install the convenient `au` binary alias:
```powershell
go install github.com/Aswanidev-vs/animeupscale/cmd/au@latest
```

### Local Build
```powershell
go build -o au.exe .
```

---

## Web GUI (Browser-Based UI)

AnimeUpscale includes a browser-based GUI for upscaling images and videos without touching the command line. A lightweight Go HTTP server wraps `au.exe` and serves a responsive web interface.

### Launch the GUI

From the project root (where `au.exe` lives):

```powershell
# Build the web server
go build -o web\server.exe .\web

# Start the server
.\web\server.exe
```

Then open **http://127.0.0.1:8080** in your browser.

### GUI Features

- **Drag & drop** file upload (images: PNG, JPG, WebP; videos: MP4, MKV, WebM, MOV, AVI)
- **Auto-detect** mode — switches between image and video pipelines based on file extension
- **Engine selector** — picks from all detected engines (Real-ESRGAN, RealSR, Anime4KCPP, waifu2x, realcugan, builtin)
- **Scale & target** — choose 2x/3x/4x upscaling or preset targets (2K, 4K)
- **Model picker** — select engine-specific models from the dropdown
- **GPU / tile-size controls** — target specific GPUs and tune VRAM usage
- **Engine-specific options** — denoise level, threads, TTA, sharpen, grayscale, video workers
- **Real-time progress** — live streaming log via Server-Sent Events (SSE)
- **Side-by-side & overlay compare** — visually compare input vs output with a draggable slider
- **One-click download** — save the upscaled result directly from the browser

### Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `127.0.0.1:8080` | HTTP listen address |
| `-au`   | auto-detect | Path to `au.exe` |
| `-workdir` | auto-detect | Project root (parent of `web/`) |

> **Note:** The server only listens on `127.0.0.1` by default. Do not expose it to the public internet — there is no authentication and uploads are arbitrary files.

---

## Quick Reference & Option Examples

### 1. List Available Engines
Detect which native backends are available on your system:
```powershell
au.exe -list-engines
```

### 2. Upscale Images

#### Real-ESRGAN (Anime-Optimized - Recommended)
Uses Vulkan GPU acceleration. Supports downscaling massive outputs to exact targets like `-target 4k` post-upscale to prevent absurdly large image files (e.g. upscaling a 7K image directly to 29K):
```powershell
au.exe -i input.png -o output.png -engine realesrgan -scale 4 -model-name realesr-animevideov3 -target 4k
```

#### RealSR (Photorealistic / Real-World Images)
Optimized for clean real-world super-resolution (only supports `-scale 4`). Use tile sizes to manage performance and prevent VRAM out-of-memory crashes on large images:
```powershell
# Upscale using default denoised model (models-DF2K_JPEG) for compressed images
au.exe -i nature.jpg -o nature-4x.jpg -engine realsr -scale 4 -tile-size 128

# Upscale using clean model (models-DF2K) for clean/high-quality photos
au.exe -i nature.jpg -o nature-4x.jpg -engine realsr -scale 4 -model-name DF2K -tile-size 128
```

#### Built-in Engine (Pure Go Fallback)
Works anywhere without external dependencies. Supports target presets (`2k` or `4k`):
```powershell
au.exe -i portrait.png -o portrait-2k.png -engine builtin -target 2k -sharpen 0.25 -grayscale
```

---

### 3. Upscale Videos
Automatically processes videos (`.mp4`, `.mkv`, `.mov`, `.avi`, `.webm`) by extracting frames, upscaling them in parallel, and muxing audio back.

#### Dedicated GPU Targeting & Concurrency (Recommended)
Integrated GPUs (like AMD Radeon CPU integrated graphics) often run out of VRAM and crash with a Vulkan `vkQueueSubmit failed -4` error. 

Use **`-gpu`** to target your dedicated GPU (like NVIDIA GeForce GTX 1650) instead, and control concurrent threads safely:

##### Fast Anime Video Upscale (NVIDIA GPU - Highly Recommended):
```powershell
au.exe -video -i kbankai.mp4 -o bankai-4k.mp4 -engine realesrgan -target 4k -scale 4 -model-name realesr-animevideov3-x4 -video-workers 2 -gpu 1
```

##### RealSR Video Upscale (High VRAM load - Use conservative settings):
```powershell
au.exe -video -i movie.mp4 -o movie-4k.mp4 -engine realsr -target 4k -scale 4 -video-workers 1 -gpu 1 -tile-size 128
```
*Note: If dedicated GPU order is mapped first, change `-gpu 1` to `-gpu 0`.*

---

## Complete CLI Flags Options

| Flag | Default | Description | Example |
|------|---------|-------------|---------|
| `-i`, `-input` | *None (Required)* | Path to input image or video | `-i image.png` |
| `-o`, `-output` | *Auto-generated* | Output file path | `-o output.png` |
| `-engine` | `realesrgan` | Engine: `auto`, `realesrgan`, `realsr`, `builtin`, `waifu2x`, `realcugan`, `anime4kcpp` | `-engine builtin` |
| `-s`, `-scale` | `2` | Upscale scale factor (`2`, `3`, `4`) | `-scale 4` |
| `-target` | *None* | Dimensions target: `2k` (longest side 2048px) or `4k` (3840px) | `-target 4k` |
| `-noise` | `0` | Denoise level for waifu2x/realcugan | `-noise 2` |
| `-gpu` | `auto` | GPU ID or processor backend. For `anime4kcpp`, use index, index combination (`opencl:0`), or raw backend `cpu` / `opencl` / `cuda`. For others, use `-1` for CPU mode | `-gpu opencl` |
| `-model-name` | *None* | Model name. (realesrgan: `realesr-animevideov3`, etc.; realsr: `DF2K`, etc.; anime4kcpp: custom CNN model like `acnet-legacy-gan` (default), `acnet-f8b8`, `artcnn-c4f32`, `fsrcnnx-f16b4`, etc. Run `ac_cli --lm` to list) | `-model-name artcnn-c4f32` |
| `-threads` | *None* | Processor threads override (e.g. `1:2:2` for external engines, single digit `-threads 4` for Anime4KCPP) | `-threads 4` |
| `-tile-size` | `0` | Divide large images into tiles to save VRAM (e.g., `128`) | `-tile-size 128` |
| `-video-workers`| `1` | Bounded goroutine worker pool size for video frame upscaling | `-video-workers 4` |
| `-benchmark` | `false` | Enables stage timer outputs and writes `bench.json` | `-benchmark` |
| `-sharpen` | `0.15` | Builtin-only unsharp amount (0 to disable) | `-sharpen 0.3` |
| `-grayscale` | `false` | Builtin-only grayscale conversion before upscaling | `-grayscale` |
| `-layers` | `visible` | PSD layer mode: `visible` (upscale only visible layers) or `all` (upscale all layers) | `-layers all` |
| `-keep-temp` | `false` | Keep temporary extracted frame folders | `-keep-temp` |

---

### 4. Upscale PSD Files (Layered Photoshop Documents)

Process Photoshop PSD files by upscaling each individual layer and reassembling them into a new PSD. Layer metadata (name, opacity, blend mode, visibility, position) is preserved and positions are scaled proportionally.

```powershell
# Upscale all visible layers (default) using Real-ESRGAN
au.exe -i design.psd -o design-upscaled.psd -engine realesrgan -scale 4

# Upscale all layers including hidden ones
au.exe -i design.psd -o design-upscaled.psd -layers all -engine builtin -scale 2

# Built-in engine with line enhancement
au.exe -i artwork.psd -o artwork-upscaled.psd -engine builtin -scale 2 -sharpen 0.25
```

**How it works:**
1. Each layer is extracted as a temporary PNG file
2. Each layer PNG is upscaled individually using the selected engine
3. Layer bounds (position/size) are scaled by the scale factor
4. All upscaled layers are reassembled into a new PSD with preserved metadata
5. A composite preview image is generated from the visible layers

**Supported features:** RGB 8-bit PSDs with RLE-compressed layers; layer names, blend modes, opacity, visibility flags, and clipping are preserved.

---



## Anime4KCPP Engine Integration Guide (`-engine anime4kcpp`)

The **Anime4KCPP** engine uses powerful CNN-based and legacy upscaling algorithms. It is natively auto-detected inside `Anime4KCPP-CLI-v3.2.0-x64-MSVC/` or a project `bin/` directory.

### Custom Backend Selection (`-gpu`)
Use the `-gpu` flag to target specific GPU computing architectures or hardware processors:
- `-gpu cpu`: General-purpose CPU processing (highly optimized with SIMD).
- `-gpu opencl`: Cross-platform OpenCL acceleration (compatible with AMD, Intel, and NVIDIA).
- `-gpu cuda`: Direct hardware acceleration on NVIDIA GPUs.
- Custom Device Index: Target a specific device (e.g., `-gpu 1` or `-gpu opencl:0`).

### Model Options (`-model-name`)
Specify modern CNNs or legacy filters. The engine supports a vast selection of models spanning different architecture profiles:
- **Legacy CNNs**:
  - `acnet-legacy-gan` (Default): High performance, detail enhancement.
  - `acnet-legacy-hdn0` to `acnet-legacy-hdn3`: Four levels of standard denoising.
- **ACNet VGG/ResNet Styles**:
  - `acnet-f8b4` to `acnet-f8b18`: Balanced networks (e.g. `acnet-f8b18-box` for sharp vector styles).
  - `arnet-f8b8` to `arnet-f8b64-box-hdn`: Deep ResNet designs from minimal to heavy denoising styles.
- **ArtCNN Premium Networks**:
  - `artcnn-c4f16` / `artcnn-c4f32`: Neutral premium CNN layers.
  - `artcnn-c4f32-dn`: Denoise and soften style.
  - `artcnn-c4f32-ds`: Denoise and sharpen style.
- **FSRCNNX Super-Resolution**:
  - `fsrcnnx-f8b4` / `fsrcnnx-f16b4`: Highly accurate reconstruction networks.
  - `fsrcnnx-f16b4-distort-plus`: Best-in-class recovery for highly compressed media.

*Tip: Run `./Anime4KCPP-CLI-v3.2.0-x64-MSVC/ac_cli.exe --lm` to view all parameters and descriptions.*

### Thread Assignment & Resource Limits for Video Upscaling (`-threads` vs `-video-workers`)
When upscaling video files, performance is determined by two separate layers of concurrency:

1. **`-video-workers` (Frame level)**: Controls the number of frames being upscaled concurrently (Go routine workers).
2. **`-threads` (Processor level)**: Controls the internal threads used by the engine for **each frame**.

#### Allocation Limits and Safety Rules
- **GPU (OpenCL / CUDA) limits**: GPUs process calls in pipelines. Setting `-video-workers` too high (e.g. $>4$) will saturate VRAM and GPU compute queues, causing crashes (`vkQueueSubmit failed`). Keep `-video-workers` between `1` and `4` depending on VRAM.
- **CPU limits**: The total assigned CPU threads must not saturate your physical CPU cores. 
  $$\text{Total CPU Threads} = (\text{video-workers}) \times (\text{threads})$$
  Keep this total strictly **under or equal to** your hardware thread limit:
  - If CPU has 16 threads: set `-video-workers 4` and `-threads 4` (total 16), or `-video-workers 2` and `-threads 8` (total 16).
  - Setting `-threads 0` tells the engine to auto-allocate based on hardware thread limits.

### Examples

#### Basic Anime4KCPP Image Upscaling:
```powershell
au.exe -i input.png -o output.png -engine anime4kcpp -scale 2 -gpu opencl
```

#### High-Quality CNN Upscale (ArtCNN):
```powershell
au.exe -i input.png -o output.png -engine anime4kcpp -scale 2 -model-name artcnn-c4f32 -gpu cuda
```

#### Vector Art / Sharp Line-Art Restoration (ACNet Box model):
```powershell
au.exe -i lines.png -o sharp-lines.png -engine anime4kcpp -scale 2 -model-name acnet-f8b18-box -gpu opencl
```

#### Legacy Denoising (ACNet GAN with Denoise lvl 2):
```powershell
au.exe -i noise.jpg -o clean.png -engine anime4kcpp -scale 2 -model-name acnet-legacy-hdn2 -gpu cpu
```

#### Heavy Artifact Recovery / Highly Compressed Media (FSRCNNX Distort Plus):
```powershell
au.exe -i artifacts.jpg -o restored.png -engine anime4kcpp -scale 2 -model-name fsrcnnx-f16b4-distort-plus -gpu cuda
```

#### Bounded Parallel Video Upscaling:
```powershell
au.exe -video -i in.mp4 -o out.mp4 -engine anime4kcpp -scale 2 -target 2k -model-name acnet-f8b18 -threads 2 -video-workers 4 -gpu opencl
```
---

## Tile Size Guide (`-tile-size`)

When upscaling, the native engine loads the entire image into GPU VRAM by default (`-tile-size 0`). For large images or limited GPUs, this causes **Vulkan out-of-memory crashes** (`vkQueueSubmit failed -4`).

Setting `-tile-size` splits the image into square blocks of that pixel size, processes them one at a time, and stitches the result back together. Smaller tiles use less VRAM but run slightly slower.

### Recommended Tile Sizes by GPU VRAM

| GPU VRAM | Tile Size | Notes |
|----------|-----------|-------|
| **2 GB** (integrated / low-end) | `64` – `128` | Safe but slow. Use `-video-workers 1`. |
| **4 GB** (GTX 1650, RX 580) | `128` – `256` | Good balance of speed and stability. |
| **6 GB** (RTX 2060, RTX 3060) | `256` – `400` | Fast. Can try `-video-workers 2`. |
| **8 GB+** (RTX 3070+, RTX 4070+) | `400` – `0` | Use `0` (no tiling) for max speed. |

### How to Choose

1. **Start with `0`** (no tiling). If it works, you get maximum speed.
2. **If you see `vkQueueSubmit failed`**, set `-tile-size 256` and retry.
3. **Still crashing?** Lower to `128` or `64`.
4. **Multiple workers?** Each worker uses its own tile buffer. With `-video-workers 2`, halve your tile size (e.g., use `128` instead of `256`).

### Examples

```powershell
# 4GB GPU, single image, safe tile size
au.exe -i photo.jpg -o photo-4x.png -engine realsr -scale 4 -tile-size 128

# 6GB GPU, video with 2 workers
au.exe -video -i clip.mp4 -o clip-4k.mp4 -engine realesrgan -target 4k -scale 4 -model-name realesr-animevideov3 -video-workers 2 -tile-size 256

# 8GB+ GPU, no tiling needed
au.exe -i wallpaper.png -o wallpaper-4x.png -engine realesrgan -scale 4 -model-name realesr-animevideov3
```

---

## Performance & Safety Optimizations
- **Fast Path Resolution Caching**: Eliminates redundant disk scans during video upscaling, reducing upscaling overhead on large files.
- **Corrupt File Protection**: If upscaling or ffmpeg encode fails, temporary/partial outputs are cleaned up automatically so your final file is never left half-written or corrupted.
- **Bounded Concurrency**: Controls memory and GPU overhead using `-video-workers`.
