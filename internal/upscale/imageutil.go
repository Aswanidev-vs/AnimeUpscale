package upscale

import (
	"fmt"
	"image"
	"os"
)

func imageBounds(path string) (image.Rectangle, error) {
	// PSD files need special handling since Go's image.DecodeConfig doesn't understand them.
	if isPSDFile(path) {
		psd, err := readPSD(path)
		if err != nil {
			return image.Rectangle{}, fmt.Errorf("read psd bounds %q: %w", path, err)
		}
		return image.Rect(0, 0, psd.Width, psd.Height), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("open image %q: %w", path, err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("read image config %q: %w", path, err)
	}
	return image.Rect(0, 0, cfg.Width, cfg.Height), nil
}
