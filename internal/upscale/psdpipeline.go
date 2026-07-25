package upscale

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// psdPipeline orchestrates PSD layer extraction, per-layer upscaling, and reassembly.
// It works like the video pipeline: wraps the Manager so any engine can process PSD files.
type psdPipeline struct {
	manager *Manager
}

func newPSDPipeline(m *Manager) *psdPipeline {
	return &psdPipeline{manager: m}
}

// Process reads a PSD, upscales each layer, and writes a new PSD.
func (p *psdPipeline) Process(req Request) (Result, error) {
	// Read the PSD
	psd, err := readPSD(req.Input)
	if err != nil {
		return Result{}, fmt.Errorf("read psd: %w", err)
	}

	// Determine which layers to process
	layerMode := req.LayerMode
	if layerMode == "" {
		layerMode = "visible"
	}

	type layerJob struct {
		orig   *psdLayer
		scaleW int
		scaleH int
	}
	var jobs []layerJob

	for _, l := range psd.Layers {
		if layerMode == "visible" && !l.Visible {
			continue
		}
		lw := int(l.Right - l.Left)
		lh := int(l.Bottom - l.Top)
		if lw <= 0 || lh <= 0 {
			continue
		}
		sW := lw * req.Scale
		sH := lh * req.Scale
		if req.TargetW > 0 && req.TargetH > 0 {
			// Scale target proportionally per layer? No — use scale factor.
		}
		jobs = append(jobs, layerJob{orig: l, scaleW: sW, scaleH: sH})
	}

	if len(jobs) == 0 {
		return Result{}, fmt.Errorf("no layers to upscale (layer mode: %s)", layerMode)
	}

	// Create temp directory for layer PNGs
	tempDir, err := os.MkdirTemp("", "au-psd-*")
	if err != nil {
		return Result{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// For each layer: extract as PNG → upscale via manager → collect result
	type jobResult struct {
		layer  *psdLayer
		outW   int
		outH   int
		outImg *image.NRGBA
		err    error
	}

	results := make([]jobResult, len(jobs))
	for i, job := range jobs {
		layerPath := filepath.Join(tempDir, fmt.Sprintf("layer_%03d.png", i))
		upscaledPath := filepath.Join(tempDir, fmt.Sprintf("layer_%03d_upscaled.png", i))

		// Write layer as PNG
		if err := writeLayerPNG(layerPath, job.orig.Data); err != nil {
			return Result{}, fmt.Errorf("write layer %d: %w", i, err)
		}

		// Build an upscale request for this layer
		layerReq := req
		layerReq.Input = layerPath
		layerReq.Output = upscaledPath
		// Use scale-based upscaling (not target), so layer dimensions grow by scale factor
		layerReq.TargetW = 0
		layerReq.TargetH = 0
		layerReq.Target = ""
		layerReq.Format = "png"

		_, err := p.manager.Upscale(layerReq)
		if err != nil {
			return Result{}, fmt.Errorf("upscale layer %d (%s): %w", i, job.orig.Name, err)
		}

		// Read back the upscaled image
		img, err := readPNG(upscaledPath)
		if err != nil {
			return Result{}, fmt.Errorf("read upscaled layer %d: %w", i, err)
		}

		outW := job.scaleW
		outH := job.scaleH
		if img.Bounds().Dx() != outW || img.Bounds().Dy() != outH {
			// The engine may produce different dimensions (e.g. forced scale multiples).
			// Use the actual upscaled dimensions.
			outW = img.Bounds().Dx()
			outH = img.Bounds().Dy()
		}

		results[i] = jobResult{
			layer:  job.orig,
			outW:   outW,
			outH:   outH,
			outImg: img,
		}
	}

	// Build new psdFile with upscaled layers
	outPSD := &psdFile{
		Width:     psd.Width * req.Scale,
		Height:    psd.Height * req.Scale,
		Channels:  psd.Channels,
		Depth:     psd.Depth,
		ColorMode: psd.ColorMode,
	}

	for _, r := range results {
		lw := r.outW
		lh := r.outH
		// Scale layer bounds proportionally
		left := r.layer.Left * int32(req.Scale)
		top := r.layer.Top * int32(req.Scale)

		outLayer := &psdLayer{
			Name:      r.layer.Name,
			Top:       top,
			Left:      left,
			Bottom:    top + int32(lh),
			Right:     left + int32(lw),
			Opacity:   r.layer.Opacity,
			Visible:   r.layer.Visible,
			BlendMode: r.layer.BlendMode,
			Clipping:  r.layer.Clipping,
			Data:      r.outImg,
		}
		outPSD.Layers = append(outPSD.Layers, outLayer)
	}

	// Write output PSD
	if err := writePSD(req.Output, outPSD); err != nil {
		return Result{}, fmt.Errorf("write output psd: %w", err)
	}

	return Result{
		Engine:       req.Engine,
		Output:       req.Output,
		InputWidth:   psd.Width,
		InputHeight:  psd.Height,
		OutputWidth:  outPSD.Width,
		OutputHeight: outPSD.Height,
		LayerCount:   len(outPSD.Layers),
		Note:         fmt.Sprintf("upscaled %d layer(s) from PSD", len(outPSD.Layers)),
	}, nil
}

func writeLayerPNG(path string, img *image.NRGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func readPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return dst, nil
}

// IsPSDFile checks whether a path appears to be a PSD file.
func IsPSDFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".psd")
}

// PSDDimensions returns the width and height of a PSD file without loading all layers.
func PSDDimensions(path string) (width, height int, err error) {
	psd, err := readPSD(path)
	if err != nil {
		return 0, 0, err
	}
	return psd.Width, psd.Height, nil
}

// package-level alias for internal use
func isPSDFile(path string) bool {
	return IsPSDFile(path)
}
