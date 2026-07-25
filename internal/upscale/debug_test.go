package upscale

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func TestCheckFileSize(t *testing.T) {
	for _, path := range []string{"E:/AnimeUpscale/mai-roundtrip.psd", "E:/AnimeUpscale/mai-upscaled.psd"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		cmLen := int(binary.BigEndian.Uint32(data[26:]))
		irLen := int(binary.BigEndian.Uint32(data[30:]))
		lmStart := 34 + irLen
		lmLen := int(binary.BigEndian.Uint32(data[lmStart:]))
		imgStart := lmStart + 4 + lmLen
		compression := binary.BigEndian.Uint16(data[imgStart:])
		remaining := len(data) - imgStart - 2
		fmt.Printf("%s: total=%d cm=%d ir=%d lmStart=%d lmLen=%d imgStart=%d compression=%d remaining=%d\n",
			path, len(data), cmLen, irLen, lmStart, lmLen, imgStart, compression, remaining)
	}
}
