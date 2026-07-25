package upscale

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func createTestPSD(t *testing.T) string {
	// Create two test layers
	layer1 := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			layer1.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	layer2 := image.NewNRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			layer2.SetNRGBA(x, y, color.NRGBA{R: 0, G: 255, B: 0, A: 128})
		}
	}

	psd := &psdFile{
		Width:     100,
		Height:    100,
		Channels:  4,
		Depth:     8,
		ColorMode: 3,
		Layers: []*psdLayer{
			{
				Name:      "Background",
				Top:       0,
				Left:      0,
				Bottom:    100,
				Right:     100,
				Opacity:   255,
				Visible:   true,
				BlendMode: "norm",
				Data:      layer1,
			},
			{
				Name:      "Overlay",
				Top:       25,
				Left:      25,
				Bottom:    75,
				Right:     75,
				Opacity:   255,
				Visible:   true,
				BlendMode: "norm",
				Data:      layer2,
			},
		},
	}

	path := filepath.Join(t.TempDir(), "test.psd")
	if err := writePSD(path, psd); err != nil {
		t.Fatalf("writePSD: %v", err)
	}
	return path
}

func TestWriteAndReadPSD(t *testing.T) {
	path := createTestPSD(t)

	// Read it back
	psd, err := readPSD(path)
	if err != nil {
		t.Fatalf("readPSD: %v", err)
	}

	if psd.Width != 100 || psd.Height != 100 {
		t.Fatalf("dimensions: got %dx%d, want 100x100", psd.Width, psd.Height)
	}
	if len(psd.Layers) != 2 {
		t.Fatalf("layer count: got %d, want 2", len(psd.Layers))
	}

	// Check layer 1
	l0 := psd.Layers[0]
	if l0.Name != "Background" {
		t.Errorf("layer 0 name: got %q, want %q", l0.Name, "Background")
	}
	if !l0.Visible {
		t.Error("layer 0 should be visible")
	}
	if l0.Data.Bounds().Dx() != 100 || l0.Data.Bounds().Dy() != 100 {
		t.Errorf("layer 0 size: got %dx%d, want 100x100", l0.Data.Bounds().Dx(), l0.Data.Bounds().Dy())
	}
	c00 := l0.Data.NRGBAAt(0, 0)
	if c00.R != 255 || c00.G != 0 || c00.B != 0 || c00.A != 255 {
		t.Errorf("layer 0 pixel (0,0): got %v, want R=255 G=0 B=0 A=255", c00)
	}

	// Check layer 2
	l1 := psd.Layers[1]
	if l1.Name != "Overlay" {
		t.Errorf("layer 1 name: got %q, want %q", l1.Name, "Overlay")
	}
	if l1.Top != 25 || l1.Left != 25 || l1.Bottom != 75 || l1.Right != 75 {
		t.Errorf("layer 1 bounds: got (%d,%d,%d,%d), want (25,25,75,75)", l1.Top, l1.Left, l1.Bottom, l1.Right)
	}
	c00 = l1.Data.NRGBAAt(0, 0)
	if c00.R != 0 || c00.G != 255 || c00.B != 0 || c00.A != 128 {
		t.Errorf("layer 1 pixel (0,0): got %v, want R=0 G=255 B=0 A=128", c00)
	}
}

func TestPSDDimensions(t *testing.T) {
	path := createTestPSD(t)

	w, h, err := PSDDimensions(path)
	if err != nil {
		t.Fatalf("PSDDimensions: %v", err)
	}
	if w != 100 || h != 100 {
		t.Fatalf("dimensions: got %dx%d, want 100x100", w, h)
	}
}

func TestIsPSDFile(t *testing.T) {
	if !IsPSDFile("test.psd") {
		t.Error("expected test.psd to be detected as PSD")
	}
	if !IsPSDFile("TEST.PSD") {
		t.Error("expected TEST.PSD to be detected as PSD")
	}
	if IsPSDFile("test.png") {
		t.Error("expected test.png to NOT be detected as PSD")
	}
}

func TestPSDPipelineBuiltin(t *testing.T) {
	path := createTestPSD(t)
	outPath := filepath.Join(t.TempDir(), "out.psd")

	manager := NewManager()
	_, err := manager.Upscale(Request{
		Input:     path,
		Output:    outPath,
		Engine:    "builtin",
		Scale:     2,
		Format:    "psd",
		LayerMode: "visible",
	})
	if err != nil {
		t.Fatalf("Upscale PSD: %v", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output PSD not created: %v", err)
	}

	// Read back and verify
	outPSD, err := readPSD(outPath)
	if err != nil {
		t.Fatalf("read output PSD: %v", err)
	}

	if outPSD.Width != 200 || outPSD.Height != 200 {
		t.Errorf("output dimensions: got %dx%d, want 200x200", outPSD.Width, outPSD.Height)
	}
	if len(outPSD.Layers) != 2 {
		t.Errorf("output layer count: got %d, want 2", len(outPSD.Layers))
	}

	// Layer 0 should be at (0,0) and 200x200
	l0 := outPSD.Layers[0]
	if l0.Top != 0 || l0.Left != 0 || l0.Bottom != 200 || l0.Right != 200 {
		t.Errorf("layer 0 bounds: got (%d,%d,%d,%d), want (0,0,200,200)", l0.Top, l0.Left, l0.Bottom, l0.Right)
	}

	// Layer 1 should be at (50,50) and 100x100
	l1 := outPSD.Layers[1]
	if l1.Top != 50 || l1.Left != 50 || l1.Bottom != 150 || l1.Right != 150 {
		t.Errorf("layer 1 bounds: got (%d,%d,%d,%d), want (50,50,150,150)", l1.Top, l1.Left, l1.Bottom, l1.Right)
	}

	// Check names preserved
	if l0.Name != "Background" || l1.Name != "Overlay" {
		t.Errorf("layer names: got %q and %q, want Background and Overlay", l0.Name, l1.Name)
	}
}
