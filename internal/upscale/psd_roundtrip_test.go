package upscale

import (
	"testing"
)

func TestPSDRoundtripToFile(t *testing.T) {
	psd, err := readPSD("E:/AnimeUpscale/mai.psd")
	if err != nil {
		t.Skip("mai.psd not found, skipping")
	}

	// Write 1:1 roundtrip (no upscale) to verify output is openable
	err = writePSD("E:/AnimeUpscale/mai-roundtrip.psd", psd)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Log("Wrote mai-roundtrip.psd (1:1 roundtrip, no upscale)")
}
