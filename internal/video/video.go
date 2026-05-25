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

	"animeupscale/internal/upscale"
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
	ModelName  string
	ModelPath  string
	GPUID      string
	TileSize   int
	Threads    string
	TTA        bool
	Noise      int
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

	fmt.Println("stage: probe")
	info, err := probeVideo(cfg.Input)
	if err != nil {
		return err
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
	defer func() {
		if !cfg.KeepTemp && success {
			_ = os.RemoveAll(workRoot)
		}
	}()

	fmt.Println("stage: extract")
	framePattern := filepath.Join(framesDir, "frame_%06d.png")
	if err := runFFmpeg("-y", "-i", cfg.Input, framePattern); err != nil {
		return fmt.Errorf("extract frames: %w", err)
	}

	frameFiles, err := filepath.Glob(filepath.Join(framesDir, "*.png"))
	if err != nil {
		return fmt.Errorf("list extracted frames: %w", err)
	}
	if len(frameFiles) == 0 {
		return fmt.Errorf("no frames extracted from input video")
	}

	fmt.Println("stage: upscale")
	scale := cfg.Scale
	if scale < 1 {
		scale = 2
	}
	for i, frame := range frameFiles {
		out := filepath.Join(upscaledDir, filepath.Base(frame))
		if _, err := p.manager.Upscale(upscale.Request{
			Input:     frame,
			Output:    out,
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
		}); err != nil {
			return fmt.Errorf("upscale frame %d: %w", i+1, err)
		}
	}

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
		return fmt.Errorf("encode video: %w", err)
	}

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
		return longest, maxInt(1, int(scale*float64(height)+0.5))
	}
	scale := float64(longest) / float64(height)
	return maxInt(1, int(scale*float64(width)+0.5)), longest
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
