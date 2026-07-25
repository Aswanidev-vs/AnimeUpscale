package cli

import (
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/Aswanidev-vs/animeupscale/internal/progress"
	"github.com/Aswanidev-vs/animeupscale/internal/upscale"
	"github.com/Aswanidev-vs/animeupscale/internal/video"
)

type config struct {
	Input        string
	Output       string
	Engine       string
	Scale        int
	Target       string
	Noise        int
	Format       string
	TileSize     int
	GPUID        string
	ModelPath    string
	ModelName    string
	Threads      string
	TTA          bool
	List         bool
	Version      bool
	Sharpen      float64
	Grayscale    bool
	Video        bool
	FPS          string
	KeepTemp     bool
	VideoCodec   string
	VideoWorkers int
	Benchmark    bool
	LayerMode    string
}

const version = "0.1.0"

func Run(args []string) error {
	cfg := config{}
	fs := flag.NewFlagSet("anime-upscaler", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&cfg.Input, "input", "", "input image path")
	fs.StringVar(&cfg.Input, "i", "", "input image path")
	fs.StringVar(&cfg.Output, "output", "", "output image path; defaults to an auto-generated file next to the input")
	fs.StringVar(&cfg.Output, "o", "", "output image path; defaults to an auto-generated file next to the input")
	fs.StringVar(&cfg.Engine, "engine", "realesrgan", "engine: auto|anime4kcpp|realsr|waifu2x|realcugan|realesrgan|builtin")
	fs.IntVar(&cfg.Scale, "scale", 2, "upscale ratio")
	fs.IntVar(&cfg.Scale, "s", 2, "upscale ratio")
	fs.StringVar(&cfg.Target, "target", "", "target preset: 2k|4k")
	fs.IntVar(&cfg.Noise, "noise", 0, "denoise level for external engines")
	fs.StringVar(&cfg.Format, "format", "", "output format: png|jpg|jpeg; defaults to output extension or input format")
	fs.IntVar(&cfg.TileSize, "tile-size", 0, "tile size for supported external engines")
	fs.StringVar(&cfg.GPUID, "gpu", "auto", "gpu id for supported external engines; use -1 for cpu")
	fs.StringVar(&cfg.ModelPath, "model-path", "", "optional external model directory")
	fs.StringVar(&cfg.ModelName, "model-name", "", "model name (realesrgan: realesr-animevideov3|realesrgan-x4plus-anime|realesrgan-x4plus; realsr: DF2K|DF2K_JPEG)")
	fs.StringVar(&cfg.Threads, "threads", "", "optional external thread tuple such as 1:2:2")
	fs.BoolVar(&cfg.TTA, "tta", false, "enable tta mode for supported external engines")
	fs.BoolVar(&cfg.List, "list-engines", false, "list detected engines")
	fs.BoolVar(&cfg.Version, "version", false, "print version")
	fs.Float64Var(&cfg.Sharpen, "sharpen", 0.15, "builtin-only unsharp amount, 0 disables")
	fs.BoolVar(&cfg.Grayscale, "grayscale", false, "builtin-only grayscale pass before upscale")
	fs.BoolVar(&cfg.Video, "video", false, "treat input as video and run the ffmpeg-based video pipeline")
	fs.StringVar(&cfg.FPS, "fps", "", "optional output fps override for video mode")
	fs.BoolVar(&cfg.KeepTemp, "keep-temp", false, "keep temp frame directories for video mode")
	fs.StringVar(&cfg.VideoCodec, "video-codec", "libx264", "video codec for video mode")
	fs.IntVar(&cfg.VideoWorkers, "video-workers", 1, "number of parallel upscaling workers for video mode (1 = sequential)")
	fs.BoolVar(&cfg.Benchmark, "benchmark", false, "print per-stage timings and write benchmark JSON (bench.json)")
	fs.StringVar(&cfg.LayerMode, "layers", "visible", "psd layer mode: visible|all")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Anime image upscaler CLI")
		fmt.Fprintln(fs.Output(), "")
		fmt.Fprintln(fs.Output(), "Examples:")
		fmt.Fprintln(fs.Output(), "  au -i in.png -o out.png -engine auto -scale 2")
		fmt.Fprintln(fs.Output(), "  au -i in.png -o out.png -engine realsr -scale 4 -model-name DF2K")
		fmt.Fprintln(fs.Output(), "  au -i in.png -o out-4k.png -engine builtin -target 4k")
		fmt.Fprintln(fs.Output(), "  au -video -i in.mp4 -o out.mp4 -engine realsr -target 4k")
		fmt.Fprintln(fs.Output(), "  au -i in.png -o out.png -engine realesrgan -scale 4 -model-name realesr-animevideov3")
		fmt.Fprintln(fs.Output(), "  au -i in.png -o out.png -engine realesrgan -scale 4 -model-name realesrgan-x4plus-anime")
		fmt.Fprintln(fs.Output(), "  au -i in.png -o out.png -engine anime4kcpp -scale 2 -model-name artcnn-c4f32 -gpu opencl")
		fmt.Fprintln(fs.Output(), "  au -i frame.jpg -o frame@4x.png -engine waifu2x -scale 4 -noise 2")
		fmt.Fprintln(fs.Output(), "  au -i design.psd -o design-upscaled.psd -engine realesrgan -scale 4")
		fmt.Fprintln(fs.Output(), "  au -i design.psd -o design-upscaled.psd -layers all -engine builtin -scale 2")
		fmt.Fprintln(fs.Output(), "  au -list-engines")
		fmt.Fprintln(fs.Output(), "")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.Version {
		fmt.Println(version)
		return nil
	}

	manager := upscale.NewManager()
	if cfg.List {
		return printEngines(manager)
	}

	if cfg.Input == "" {
		fs.Usage()
		return errors.New("input is required")
	}
	if cfg.Scale < 1 {
		return errors.New("scale must be >= 1")
	}
	if cfg.Video || isVideoFile(cfg.Input) {
		if cfg.Output == "" {
			cfg.Output = defaultVideoOutputPath(cfg.Input, cfg.Target)
		}
		if cfg.VideoWorkers < 1 {
			return errors.New("video-workers must be >= 1")
		}

		processor := video.NewProcessor(manager)
		return processor.Process(video.Config{
			Input:      cfg.Input,
			Output:     cfg.Output,
			Engine:     strings.ToLower(cfg.Engine),
			Scale:      cfg.Scale,
			Target:     strings.ToLower(strings.TrimSpace(cfg.Target)),
			FPS:        cfg.FPS,
			KeepTemp:   cfg.KeepTemp,
			VideoCodec: cfg.VideoCodec,
			Workers:    cfg.VideoWorkers,
			ModelName:  cfg.ModelName,
			ModelPath:  cfg.ModelPath,
			GPUID:      cfg.GPUID,
			TileSize:   cfg.TileSize,
			Threads:    cfg.Threads,
			TTA:        cfg.TTA,
			Noise:      cfg.Noise,
			Benchmark:  cfg.Benchmark,
		})
	}

	targetW, targetH, err := resolveTarget(cfg.Input, cfg.Target)
	if err != nil {
		return err
	}
	if targetW > 0 && targetH > 0 {
		engine := strings.ToLower(cfg.Engine)
		if engine == "auto" {
			cfg.Engine = "builtin"
		}
	}

	cfg.Output, err = resolveOutputPath(cfg.Input, cfg.Output, cfg.Target, cfg.Format)
	if err != nil {
		return err
	}

	req := upscale.Request{
		Input:     cfg.Input,
		Output:    cfg.Output,
		Engine:    strings.ToLower(cfg.Engine),
		Scale:     cfg.Scale,
		TargetW:   targetW,
		TargetH:   targetH,
		Target:    cfg.Target,
		Noise:     cfg.Noise,
		Format:    normalizeFormat(cfg.Format, cfg.Output),
		TileSize:  cfg.TileSize,
		GPUID:     cfg.GPUID,
		ModelPath: cfg.ModelPath,
		ModelName: cfg.ModelName,
		Threads:   cfg.Threads,
		TTA:       cfg.TTA,
		Sharpen:   cfg.Sharpen,
		Grayscale: cfg.Grayscale,
		LayerMode: cfg.LayerMode,
	}

	if err := os.MkdirAll(filepath.Dir(req.Output), 0o755); err != nil && filepath.Dir(req.Output) != "." {
		return fmt.Errorf("create output directory: %w", err)
	}

	spinner := progress.NewSpinner("stage: upscale")
	spinner.Start()
	result, err := manager.Upscale(req)
	spinner.Stop()
	if err != nil {
		return err
	}

	fmt.Printf("engine: %s\n", result.Engine)
	fmt.Printf("output: %s\n", result.Output)
	fmt.Printf("size: %dx%d -> %dx%d\n", result.InputWidth, result.InputHeight, result.OutputWidth, result.OutputHeight)
	if result.LayerCount > 0 {
		fmt.Printf("layers: %d\n", result.LayerCount)
	}
	if result.Note != "" {
		fmt.Printf("note: %s\n", result.Note)
	}
	return nil
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mkv", ".mov", ".avi", ".webm":
		return true
	default:
		return false
	}
}

func defaultVideoOutputPath(inputPath, target string) string {
	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	suffix := "upscaled"
	if target != "" {
		suffix = strings.ToLower(strings.TrimSpace(target))
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s.mp4", base, suffix))
}

func printEngines(manager *upscale.Manager) error {
	engines := manager.Available()
	if len(engines) == 0 {
		fmt.Println("No engines detected.")
		return nil
	}
	for _, engine := range engines {
		fmt.Printf("%-12s %s\n", engine.Name, engine.Status)
	}
	return nil
}

func normalizeFormat(format, output string) string {
	format = strings.ToLower(strings.TrimPrefix(format, "."))
	if format != "" {
		return format
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(output), "."))
	if ext == "jpg" || ext == "jpeg" {
		return "jpg"
	}
	if ext == "png" {
		return "png"
	}
	if ext == "psd" {
		return "psd"
	}
	return ""
}

func resolveTarget(inputPath, target string) (int, int, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "":
		return 0, 0, nil
	case "2k":
		return targetFromLongestSide(inputPath, 2048)
	case "4k":
		return targetFromLongestSide(inputPath, 3840)
	default:
		return 0, 0, fmt.Errorf("unsupported target %q, use 2k or 4k", target)
	}
}

func targetFromLongestSide(path string, longestSide int) (int, int, error) {
	// PSD files need special handling
	if upscale.IsPSDFile(path) {
		w, h, err := upscale.PSDDimensions(path)
		if err != nil {
			return 0, 0, fmt.Errorf("read psd for target sizing: %w", err)
		}
		cfg := image.Config{Width: w, Height: h}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return 0, 0, errors.New("invalid psd dimensions")
		}
		if cfg.Width >= cfg.Height {
			scale := float64(longestSide) / float64(cfg.Width)
			return longestSide, max(1, int(scale*float64(cfg.Height)+0.5)), nil
		}
		scale := float64(longestSide) / float64(cfg.Height)
		return max(1, int(scale*float64(cfg.Width)+0.5)), longestSide, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open input for target sizing: %w", err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("decode input for target sizing: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, errors.New("invalid input dimensions")
	}

	if cfg.Width >= cfg.Height {
		scale := float64(longestSide) / float64(cfg.Width)
		return longestSide, max(1, int(scale*float64(cfg.Height)+0.5)), nil
	}

	scale := float64(longestSide) / float64(cfg.Height)
	return max(1, int(scale*float64(cfg.Width)+0.5)), longestSide, nil
}

func resolveOutputPath(inputPath, outputPath, target, format string) (string, error) {
	if outputPath != "" {
		return outputPath, nil
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(inputPath), "."))
	switch strings.ToLower(strings.TrimPrefix(format, ".")) {
	case "jpg", "jpeg":
		ext = "jpg"
	case "png":
		ext = "png"
	case "psd":
		ext = "psd"
	}
	if ext == "" {
		ext = "png"
	}

	suffix := "upscaled"
	if target != "" {
		suffix = strings.ToLower(strings.TrimSpace(target))
	}

	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return filepath.Join(dir, fmt.Sprintf("%s-%s.%s", base, suffix, ext)), nil
}
