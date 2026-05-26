package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Aswanidev-vs/animeupscale/internal/upscale"
)

type Config struct {
	Input      string
	Output     string
	Engine     string
	Scale      int
	Target     string
	FPS        string
	KeepTemp   bool
	VideoCodec string

	Workers int

	ModelName string
	ModelPath string
	GPUID     string
	TileSize  int
	Threads   string
	TTA       bool
	Noise     int

	Benchmark bool
}

type Processor struct {
	manager *upscale.Manager
}

type probeData struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
}

type videoInfo struct {
	Width    int
	Height   int
	FPS      string
	HasAudio bool
}

func NewProcessor(manager *upscale.Manager) *Processor {
	return &Processor{manager: manager}
}

func (p *Processor) Process(cfg Config) error {
	if err := requireBinary("ffmpeg"); err != nil {
		return err
	}
	if err := requireBinary("ffprobe"); err != nil {
		return err
	}

	engine := strings.ToLower(cfg.Engine)
	if engine == "" || engine == "auto" {
		engine = "realesrgan"
	}
	if engine == "builtin" {
		return fmt.Errorf("video mode does not support builtin engine; use realsr or another native backend")
	}
	if cfg.Target != "2k" && cfg.Target != "4k" {
		return fmt.Errorf("video mode requires -target 2k or -target 4k")
	}
	if ok, detail := p.manager.IsAvailable(engine); !ok {
		return fmt.Errorf("%s is unavailable for video mode: %s", engine, detail)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil && filepath.Dir(cfg.Output) != "." {
		return fmt.Errorf("create output directory: %w", err)
	}

	var probeStart time.Time
	if cfg.Benchmark {
		probeStart = time.Now()
	}

	stages := map[string]any{}
	fmt.Println("stage: probe")
	info, err := probeVideo(cfg.Input)
	if err != nil {
		return err
	}
	if cfg.Benchmark {
		stages["probe_seconds"] = time.Since(probeStart).Seconds()
		fmt.Printf("benchmark: probe %.3fs\n", stages["probe_seconds"].(float64))
	}

	jobID := sanitizeBaseName(strings.TrimSuffix(filepath.Base(cfg.Input), filepath.Ext(cfg.Input)))
	workRoot := filepath.Join("temp", jobID)
	framesDir := filepath.Join(workRoot, "frames")
	upscaledDir := filepath.Join(workRoot, "upscaled")

	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return fmt.Errorf("create frames dir: %w", err)
	}
	if err := os.MkdirAll(upscaledDir, 0o755); err != nil {
		return fmt.Errorf("create upscaled dir: %w", err)
	}
	success := false

	type benchData struct {
		Input   string `json:"input"`
		Output  string `json:"output"`
		Engine  string `json:"engine"`
		Scale   int    `json:"scale"`
		Target  string `json:"target"`
		Workers int    `json:"workers"`

		ModelName string `json:"model_name,omitempty"`
		ModelPath string `json:"model_path,omitempty"`
		GPUID     string `json:"gpu_id,omitempty"`
		TileSize  int    `json:"tile_size,omitempty"`
		Threads   string `json:"threads,omitempty"`
		TTA       bool   `json:"tta"`
		Noise     int    `json:"noise,omitempty"`

		Frames       int            `json:"frames,omitempty"`
		Stages       map[string]any `json:"stages"`
		TotalSeconds float64        `json:"total_seconds"`
	}

	var bench benchData
	var totalStart time.Time

	bench.Stages = stages

	if cfg.Benchmark {
		totalStart = time.Now()
		bench.Input = cfg.Input
		bench.Output = cfg.Output
		bench.Engine = engine
		bench.Scale = cfg.Scale
		bench.Target = cfg.Target
		bench.Workers = cfg.Workers
		bench.ModelName = cfg.ModelName
		bench.ModelPath = cfg.ModelPath
		bench.GPUID = cfg.GPUID
		bench.TileSize = cfg.TileSize
		bench.Threads = cfg.Threads
		bench.TTA = cfg.TTA
		bench.Noise = cfg.Noise
	}

	defer func() {
		if !cfg.KeepTemp && success {
			_ = os.RemoveAll(workRoot)
		}
		if cfg.Benchmark {
			bench.TotalSeconds = time.Since(totalStart).Seconds()

			out, _ := json.MarshalIndent(bench, "", "  ")
			_ = os.WriteFile("bench.json", out, 0o644)
		}
	}()

	var extractStart time.Time
	if cfg.Benchmark {
		extractStart = time.Now()
	}
	fmt.Println("stage: extract")
	framePattern := filepath.Join(framesDir, "frame_%06d.png")
	if err := runFFmpeg("-y", "-i", cfg.Input, framePattern); err != nil {
		return fmt.Errorf("extract frames: %w", err)
	}
	if cfg.Benchmark {
		stages["extract_seconds"] = time.Since(extractStart).Seconds()
		fmt.Printf("benchmark: extract %.3fs\n", stages["extract_seconds"].(float64))
	}

	frameFiles, err := filepath.Glob(filepath.Join(framesDir, "*.png"))
	if err != nil {
		return fmt.Errorf("list extracted frames: %w", err)
	}
	if len(frameFiles) == 0 {
		return fmt.Errorf("no frames extracted from input video")
	}

	var upscaleStart time.Time
	fmt.Println("stage: upscale")
	if cfg.Benchmark {
		upscaleStart = time.Now()
	}
	scale := cfg.Scale
	if scale < 1 {
		scale = 2
	}

	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}
	total := len(frameFiles)
	bench.Frames = total

	// Ensure stable filenames order in case Glob returns unsorted results.
	// We rely on frame_%06d.png naming; parse the numeric suffix.
	type frameJob struct {
		idx   int
		path  string
		out   string
		order int
	}
	jobs := make([]frameJob, 0, total)
	for _, frame := range frameFiles {
		base := filepath.Base(frame) // frame_000001.png
		num := 0
		if strings.HasPrefix(base, "frame_") && strings.HasSuffix(base, ".png") {
			s := strings.TrimSuffix(strings.TrimPrefix(base, "frame_"), ".png")
			parsed, err := strconv.Atoi(s)
			if err == nil {
				num = parsed
			}
		}
		jobs = append(jobs, frameJob{
			idx:   len(jobs),
			path:  frame,
			out:   filepath.Join(upscaledDir, base),
			order: num,
		})
	}

	// Simple insertion sort by order (no extra deps).
	for i := 1; i < len(jobs); i++ {
		j := jobs[i]
		k := i - 1
		for k >= 0 && jobs[k].order > j.order {
			jobs[k+1] = jobs[k]
			k--
		}
		jobs[k+1] = j
	}

	progressBar := func(done, total int) {
		width := 30
		p := float64(done) / float64(total)
		filled := int(p * float64(width))
		if filled > width {
			filled = width
		}
		empty := width - filled
		// carriage return so it updates in-place
		// IMPORTANT: do NOT print newline when upscale reaches 100%.
		// The newline should only happen after:
		//   stage: encode
		//   stage: mux audio
		fmt.Printf("\r[%s%s] %3d/%d (%.1f%%)", strings.Repeat("=", filled), strings.Repeat(" ", empty), done, total, p*100)
	}

	finishProgress := func() {
		// Ensure cursor moves to the next line after encode/mux completes.
		fmt.Printf("\n")
	}

	// Worker pool
	type jobResult struct {
		jobIdx int
		err    error
	}
	jobCh := make(chan frameJob)
	resCh := make(chan jobResult, workers)

	done := 0
	var doneMu sync.Mutex
	var once sync.Once
	var firstErr error

	progressBar(0, total)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				_, err := p.manager.Upscale(upscale.Request{
					Input:     job.path,
					Output:    job.out,
					Engine:    engine,
					Scale:     scale,
					Format:    "png",
					ModelName: cfg.ModelName,
					ModelPath: cfg.ModelPath,
					GPUID:     cfg.GPUID,
					TileSize:  cfg.TileSize,
					Threads:   cfg.Threads,
					TTA:       cfg.TTA,
					Noise:     cfg.Noise,
				})
				resCh <- jobResult{jobIdx: job.idx, err: err}
			}
		}()
	}

	// Enqueue jobs
	go func() {
		for _, job := range jobs {
			jobCh <- job
		}
		close(jobCh)
	}()

	// Collect results
	for i := 0; i < total; i++ {
		r := <-resCh
		if r.err != nil {
			once.Do(func() { firstErr = r.err })
		}
		doneMu.Lock()
		done++
		currDone := done
		doneMu.Unlock()
		progressBar(currDone, total)
	}

	wg.Wait()

	if firstErr != nil {
		return fmt.Errorf("upscale failed: %w", firstErr)
	}
	if cfg.Benchmark {
		stages["upscale_seconds"] = time.Since(upscaleStart).Seconds()
		fmt.Printf("benchmark: upscale %.3fs\n", stages["upscale_seconds"].(float64))
	}

	// Upscale is complete; show final 100% but keep progress bar line “open”
	// until encode + mux finish.
	progressBar(total, total)

	targetW, targetH := targetDimensions(info.Width*cfg.Scale, info.Height*cfg.Scale, cfg.Target)
	fps := cfg.FPS
	if fps == "" {
		fps = info.FPS
	}
	if fps == "" {
		fps = "24"
	}
	codec := cfg.VideoCodec
	if codec == "" {
		codec = "libx264"
	}

	var encodeStart time.Time
	if cfg.Benchmark {
		encodeStart = time.Now()
	}
	fmt.Println("stage: encode")
	upscaledPattern := filepath.Join(upscaledDir, "frame_%06d.png")
	args := []string{
		"-y",
		"-framerate", fps,
		"-i", upscaledPattern,
		"-i", cfg.Input,
		"-vf", fmt.Sprintf("scale=%d:%d:flags=lanczos", targetW, targetH),
		"-c:v", codec,
		"-pix_fmt", "yuv420p",
		"-map", "0:v:0",
	}
	if info.HasAudio {
		fmt.Println("stage: mux audio")
		args = append(args, "-map", "1:a?", "-c:a", "copy", "-shortest")
	}
	args = append(args, cfg.Output)
	if err := runFFmpeg(args...); err != nil {
		_ = os.Remove(cfg.Output) // prevent corrupted file
		return fmt.Errorf("encode video: %w", err)
	}
	if cfg.Benchmark {
		// include mux audio inside the encode ffmpeg call when audio exists
		stages["encode_seconds"] = time.Since(encodeStart).Seconds()

		fmt.Printf("benchmark: encode/mux %.3fs\n", stages["encode_seconds"].(float64))
	}

	// encode finished (and mux audio stage may have run above)
	finishProgress()

	success = true
	return nil
}

func probeVideo(path string) (videoInfo, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json", path)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return videoInfo{}, fmt.Errorf("ffprobe failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}

	var data probeData
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return videoInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}

	info := videoInfo{}
	for _, stream := range data.Streams {
		switch stream.CodecType {
		case "video":
			if info.Width == 0 {
				info.Width = stream.Width
				info.Height = stream.Height
				info.FPS = normalizeFPS(stream.AvgFrameRate)
				if info.FPS == "" {
					info.FPS = normalizeFPS(stream.RFrameRate)
				}
			}
		case "audio":
			info.HasAudio = true
		}
	}
	if info.Width == 0 || info.Height == 0 {
		return videoInfo{}, fmt.Errorf("no video stream found in %s", path)
	}
	return info, nil
}

func normalizeFPS(raw string) string {
	if raw == "" || raw == "0/0" {
		return ""
	}
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return raw
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return raw
	}
	return strconv.FormatFloat(num/den, 'f', 6, 64)
}

func runFFmpeg(args ...string) error {
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func requireBinary(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s is required but was not found in PATH", name)
	}
	return nil
}

func targetDimensions(width, height int, target string) (int, int) {
	longest := 2048
	if strings.EqualFold(target, "4k") {
		longest = 3840
	}
	if width >= height {
		scale := float64(longest) / float64(width)
		return longest, max(1, int(scale*float64(height)+0.5))
	}
	scale := float64(longest) / float64(height)
	return max(1, int(scale*float64(width)+0.5)), longest
}

func sanitizeBaseName(name string) string {
	name = strings.ToLower(name)
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_")
	name = replacer.Replace(name)
	if name == "" {
		return "video_job"
	}
	return name
}

