package upscale

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"animeupscale/internal/nativeexec"
)

type externalEngine struct {
	name     string
	binaries []string
	builder  func(req Request, binary string) []string
	note     string
	locate   func() (string, string, bool, string)
	runner   func(binary string, args []string) ([]byte, error)
}

func (e externalEngine) Name() string {
	return e.name
}

func (e externalEngine) Available() (bool, string) {
	if e.locate != nil {
		if path, _, ok, detail := e.locate(); ok {
			return true, fmt.Sprintf("%s (%s)", path, detail)
		}
	}
	for _, candidate := range e.binaries {
		if path, err := exec.LookPath(candidate); err == nil {
			return true, path
		}
	}
	return false, "binary not found in PATH"
}

func (e externalEngine) Upscale(req Request) (Result, error) {
	inBounds, err := imageBounds(req.Input)
	if err != nil {
		return Result{}, err
	}
	if e.name == "realesrgan" {
		if err := validateRealESRGAN(req); err != nil {
			return Result{}, err
		}
	}

	binary, err := e.binaryPath()
	if err != nil {
		return Result{}, err
	}
	args := e.builder(req, binary)
	output, err := e.run(binary, args)
	if err != nil {
		return Result{}, fmt.Errorf("%s failed: %w\n%s", e.name, err, string(output))
	}

	outBounds, err := imageBounds(req.Output)
	if err != nil {
		outBounds = image.Rect(0, 0, inBounds.Dx()*req.Scale, inBounds.Dy()*req.Scale)
	}

	return Result{
		Engine:       e.name,
		Output:       req.Output,
		InputWidth:   inBounds.Dx(),
		InputHeight:  inBounds.Dy(),
		OutputWidth:  outBounds.Dx(),
		OutputHeight: outBounds.Dy(),
		Note:         e.note,
	}, nil
}

func (e externalEngine) binaryPath() (string, error) {
	if e.locate != nil {
		if path, _, ok, _ := e.locate(); ok {
			return path, nil
		}
	}
	for _, candidate := range e.binaries {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s binary not found", e.name)
}

func (e externalEngine) run(binary string, args []string) ([]byte, error) {
	if e.runner != nil {
		return e.runner(binary, args)
	}
	cmd := exec.Command(binary, args...)
	return cmd.CombinedOutput()
}

func NewRealESRGANEngine() Engine {
	return externalEngine{
		name:     "realesrgan",
		binaries: []string{"realesrgan-ncnn-vulkan.exe", "realesrgan-ncnn-vulkan"},
		note:     "wrapped Real-ESRGAN ncnn Vulkan engine (default: realesr-animevideov3; use -model-name realesrgan-x4plus-anime for 6B variant)",
		locate:   locateRealESRGAN,
		// Omit runner to use default exec.Command, avoiding
		// CWD issues when the binary is in a subdirectory like bin/
		builder: func(req Request, _ string) []string {
			scale := req.Scale
			if scale < 2 {
				scale = 2
			}
			if scale > 4 {
				scale = 4
			}

			modelPath := req.ModelPath
			modelName := req.ModelName
			_, repoModels, _, _ := locateRealESRGAN()
			if modelPath == "" {
				modelPath = repoModels
			}
			if modelName == "" {
				modelName = "realesr-animevideov3"
			}
			// Map convenience aliases to actual ncnn model names
			switch modelName {
			case "realesrgan-x4plus-anime-6b", "RealESRGAN_x4plus_anime_6B", "realesrgan-6b":
				modelName = "realesrgan-x4plus-anime"
			case "realesrgan-x4plus", "realesrgan-4x":
				modelName = "realesrgan-x4plus"
			}

			args := []string{
				"-i", req.Input,
				"-o", req.Output,
				"-s", strconv.Itoa(scale),
				"-f", normalizeExternalFormat(req.Format),
			}
			if modelPath != "" {
				args = append(args, "-m", modelPath)
			}
			if modelName != "" {
				args = append(args, "-n", modelName)
			}
			if req.TileSize > 0 {
				args = append(args, "-t", strconv.Itoa(req.TileSize))
			}
			if req.GPUID != "" && req.GPUID != "auto" {
				args = append(args, "-g", req.GPUID)
			}
			if req.Threads != "" {
				args = append(args, "-j", req.Threads)
			}
			if req.TTA {
				args = append(args, "-x")
			}
			return args
		},
	}
}

func NewRealSREngine() Engine {
	return externalEngine{
		name:     "realsr",
		binaries: []string{"realsr-ncnn-vulkan.exe", "realsr-ncnn-vulkan"},
		note:     "wrapped RealSR ncnn Vulkan engine via cgo runner",
		locate:   locateRealSR,
		runner:   nativeexec.Run,
		builder: func(req Request, _ string) []string {
			scale := req.Scale
			if scale != 4 {
				scale = 4
			}

			modelPath := req.ModelPath
			_, repoModels, _, _ := locateRealSR()
			if modelPath == "" {
				modelPath = repoModels
			}

			args := []string{
				"-i", req.Input,
				"-o", req.Output,
				"-s", strconv.Itoa(scale),
				"-f", normalizeExternalFormat(req.Format),
			}
			if modelPath != "" {
				args = append(args, "-m", modelPath)
			}
			if req.TileSize > 0 {
				args = append(args, "-t", strconv.Itoa(req.TileSize))
			}
			if req.GPUID != "" && req.GPUID != "auto" {
				args = append(args, "-g", req.GPUID)
			}
			if req.Threads != "" {
				args = append(args, "-j", req.Threads)
			}
			if req.TTA {
				args = append(args, "-x")
			}
			return args
		},
	}
}

func NewAnime4KCPPEngine() Engine {
	return externalEngine{
		name:     "anime4kcpp",
		binaries: []string{"ac_cli.exe", "ac_cli"},
		note:     "wrapped external Anime4KCPP CLI",
		builder: func(req Request, _ string) []string {
			args := []string{
				"-i", req.Input,
				"-o", req.Output,
				"-z", strconv.Itoa(req.Scale),
			}
			if req.GPUID == "-1" {
				args = append(args, "-q")
			}
			return args
		},
	}
}

func NewWaifu2xEngine() Engine {
	return externalEngine{
		name:     "waifu2x",
		binaries: []string{"waifu2x-ncnn-vulkan.exe", "waifu2x-ncnn-vulkan"},
		note:     "wrapped external waifu2x-ncnn-vulkan",
		builder: func(req Request, _ string) []string {
			args := []string{
				"-i", req.Input,
				"-o", req.Output,
				"-n", strconv.Itoa(req.Noise),
				"-s", strconv.Itoa(req.Scale),
				"-f", normalizeExternalFormat(req.Format),
			}
			if req.TileSize > 0 {
				args = append(args, "-t", strconv.Itoa(req.TileSize))
			}
			if req.ModelPath != "" {
				args = append(args, "-m", req.ModelPath)
			}
			if req.GPUID != "" && req.GPUID != "auto" {
				args = append(args, "-g", req.GPUID)
			}
			if req.Threads != "" {
				args = append(args, "-j", req.Threads)
			}
			if req.TTA {
				args = append(args, "-x")
			}
			return args
		},
	}
}

func NewRealCUGANEngine() Engine {
	return externalEngine{
		name:     "realcugan",
		binaries: []string{"realcugan-ncnn-vulkan.exe", "realcugan-ncnn-vulkan"},
		note:     "wrapped external realcugan-ncnn-vulkan",
		builder: func(req Request, _ string) []string {
			args := []string{
				"-i", req.Input,
				"-o", req.Output,
				"-n", strconv.Itoa(req.Noise),
				"-s", strconv.Itoa(req.Scale),
				"-f", normalizeExternalFormat(req.Format),
			}
			if req.TileSize > 0 {
				args = append(args, "-t", strconv.Itoa(req.TileSize))
			}
			if req.ModelPath != "" {
				args = append(args, "-m", req.ModelPath)
			}
			if req.GPUID != "" && req.GPUID != "auto" {
				args = append(args, "-g", req.GPUID)
			}
			if req.Threads != "" {
				args = append(args, "-j", req.Threads)
			}
			if req.TTA {
				args = append(args, "-x")
			}
			return args
		},
	}
}

func normalizeExternalFormat(format string) string {
	switch format {
	case "jpeg":
		return "jpg"
	case "jpg", "png", "webp":
		return format
	default:
		return "png"
	}
}

func locateRealESRGAN() (binaryPath, modelPath string, ok bool, detail string) {
	for _, candidate := range []string{"realesrgan-ncnn-vulkan.exe", "realesrgan-ncnn-vulkan"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, findRealESRGANModels("."), true, "PATH"
		}
		// Also check current directory
		if fileExists(candidate) {
			model := findRealESRGANModels(".")
			return filepath.Clean(candidate), model, true, "workspace"
		}
	}

	roots := []string{
		".",
		"bin",
		"realesrgan-ncnn-vulkan-v0.2.0-windows",
		"realesrgan-ncnn-vulkan-20220424-windows",
		"Real-ESRGAN-ncnn-vulkan",
		filepath.Join("Real-ESRGAN-ncnn-vulkan", "build", "install", "bin"),
		filepath.Join("Real-ESRGAN-ncnn-vulkan", "build", "bin"),
	}
	for _, root := range roots {
		for _, name := range []string{"realesrgan-ncnn-vulkan.exe", "realesrgan-ncnn-vulkan"} {
			path := filepath.Clean(filepath.Join(root, name))
			if fileExists(path) {
				model := findRealESRGANModels(root)
				if model == "" {
					model = findRealESRGANModels(".")
				}
				return path, model, true, "workspace"
			}
		}
	}

	model := findRealESRGANModels(".")
	if model != "" {
		return "", model, false, "models found but executable is missing"
	}
	return "", "", false, "binary not found in PATH or Real-ESRGAN-ncnn-vulkan workspace"
}

func locateRealSR() (binaryPath, modelPath string, ok bool, detail string) {
	for _, candidate := range []string{"realsr-ncnn-vulkan.exe", "realsr-ncnn-vulkan"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, findRealSRModels("."), true, "PATH"
		}
	}

	roots := []string{
		"bin",
		"realsr-ncnn-vulkan-20220728-windows",
	}
	for _, root := range roots {
		for _, name := range []string{"realsr-ncnn-vulkan.exe", "realsr-ncnn-vulkan"} {
			path := filepath.Clean(filepath.Join(root, name))
			if fileExists(path) {
				model := findRealSRModels(root)
				if model == "" {
					model = findRealSRModels(".")
				}
				return path, model, true, "workspace"
			}
		}
	}

	model := findRealSRModels(".")
	if model != "" {
		return "", model, false, "models found but executable is missing"
	}
	return "", "", false, "binary not found in PATH or realsr-ncnn-vulkan workspace"
}

func findRealESRGANModels(root string) string {
	candidates := []string{
		filepath.Join(root, "models"),
		filepath.Join(root, "realesrgan-ncnn-vulkan-v0.2.0-windows", "models"),
		filepath.Join(root, "realesrgan-ncnn-vulkan-20220424-windows", "models"),
		filepath.Join(root, "Real-ESRGAN-ncnn-vulkan", "models"),
		filepath.Join(root, "realesrgan-ncnn-vulkan", "models"),
	}
	for _, dir := range candidates {
		if realESRGANModelDir(dir) {
			return filepath.Clean(dir)
		}
	}
	return ""
}

func findRealSRModels(root string) string {
	candidates := []string{
		filepath.Join(root, "models", "realsr", "models-DF2K_JPEG"),
		filepath.Join(root, "models", "realsr", "models-DF2K"),
		filepath.Join(root, "models", "models-DF2K_JPEG"),
		filepath.Join(root, "models", "models-DF2K"),
		filepath.Join(root, "models-DF2K_JPEG"),
		filepath.Join(root, "models-DF2K"),
		filepath.Join(root, "realsr", "models-DF2K_JPEG"),
		filepath.Join(root, "realsr", "models-DF2K"),
		filepath.Join(root, "models", "realsr", "models-DF2K_JPEG"),
		filepath.Join(root, "models", "realsr", "models-DF2K"),
		filepath.Join(root, "realsr-ncnn-vulkan-20220728-windows", "models", "models-DF2K_JPEG"),
		filepath.Join(root, "realsr-ncnn-vulkan-20220728-windows", "models", "models-DF2K"),
		filepath.Join(root, "realsr-ncnn-vulkan-20220728-windows", "models-DF2K_JPEG"),
		filepath.Join(root, "realsr-ncnn-vulkan-20220728-windows", "models-DF2K"),
	}
	for _, dir := range candidates {
		if realSRModelDir(dir) {
			return filepath.Clean(dir)
		}
	}
	return ""
}

func realESRGANModelDir(dir string) bool {
	names := []string{
		"realesr-animevideov3-x2.param",
		"realesr-animevideov3-x3.param",
		"realesr-animevideov3-x4.param",
		"realesrgan-x4plus.param",
		"realesrgan-x4plus-anime.param",
		"realesrnet-x4plus.param",
	}
	for _, name := range names {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func realSRModelDir(dir string) bool {
	return fileExists(filepath.Join(dir, "x4.param")) && fileExists(filepath.Join(dir, "x4.bin"))
}

func validateRealESRGAN(req Request) error {
	modelPath := req.ModelPath
	if modelPath == "" {
		_, detectedModels, _, _ := locateRealESRGAN()
		modelPath = detectedModels
	}
	if modelPath == "" {
		return fmt.Errorf("realesrgan models not found; add a models folder with .param/.bin files and optionally pass -model-path")
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
