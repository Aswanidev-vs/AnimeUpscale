package upscale

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
)

// psdLayer represents a single layer from a PSD file.
type psdLayer struct {
	Name      string
	Top       int32
	Left      int32
	Bottom    int32
	Right     int32
	Opacity   byte   // 0–255
	Visible   bool
	BlendMode string // 4-char key: "norm", "dark", etc.
	Clipping  byte
	Data      *image.NRGBA
}

// psdFile holds the parsed contents of a PSD file.
type psdFile struct {
	Width     int
	Height    int
	Channels  int // 3=RGB, 4=RGBA
	Depth     int
	ColorMode int // 3=RGB
	Layers    []*psdLayer
}

// channelInfo is a parsed channel descriptor found inside each layer record.
type channelInfo struct {
	id     int16
	length uint32
}

// ---------------------------------------------------------------------------
// PSD reader
// ---------------------------------------------------------------------------

func readPSD(path string) (*psdFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open psd: %w", err)
	}
	defer f.Close()

	psd, err := parsePSD(f)
	if err != nil {
		return nil, fmt.Errorf("parse psd %q: %w", path, err)
	}
	return psd, nil
}

func parsePSD(r io.ReadSeeker) (*psdFile, error) {
	// ---------- file header (26 bytes) ----------
	var sig [4]byte
	if _, err := io.ReadFull(r, sig[:]); err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}
	if string(sig[:]) != "8BPS" {
		return nil, fmt.Errorf("not a PSD file (signature: %q)", sig)
	}

	var ver [2]byte
	if _, err := io.ReadFull(r, ver[:]); err != nil {
		return nil, fmt.Errorf("version: %w", err)
	}
	if binary.BigEndian.Uint16(ver[:]) != 1 {
		return nil, fmt.Errorf("unsupported version %d", binary.BigEndian.Uint16(ver[:]))
	}

	// reserved 6 bytes
	if _, err := io.ReadFull(r, make([]byte, 6)); err != nil {
		return nil, fmt.Errorf("reserved: %w", err)
	}

	psd := &psdFile{}

	var buf [4]byte

	readU16 := func() (int, error) {
		if _, err := io.ReadFull(r, buf[:2]); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint16(buf[:2])), nil
	}
	readU32 := func() (int, error) {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint32(buf[:])), nil
	}
	skip := func(n int) error {
		_, err := io.ReadFull(r, make([]byte, n))
		return err
	}

	psd.Channels, _ = readU16()
	psd.Height, _ = readU32()
	psd.Width, _ = readU32()
	psd.Depth, _ = readU16()
	psd.ColorMode, _ = readU16()

	if psd.Depth != 8 {
		return nil, fmt.Errorf("only 8-bit depth supported, got %d", psd.Depth)
	}
	if psd.ColorMode != 3 {
		return nil, fmt.Errorf("only RGB (mode 3) supported, got %d", psd.ColorMode)
	}

	// ---------- color mode data section ----------
	cmLen, _ := readU32()
	if cmLen > 0 {
		if err := skip(cmLen); err != nil {
			return nil, fmt.Errorf("color mode data: %w", err)
		}
	}

	// ---------- image resources section ----------
	irLen, _ := readU32()
	if irLen > 0 {
		if err := skip(irLen); err != nil {
			return nil, fmt.Errorf("image resources: %w", err)
		}
	}

	// ---------- layer and mask information section ----------
	lmLen, _ := readU32()
	if lmLen > 0 {
		lmData := make([]byte, lmLen)
		if _, err := io.ReadFull(r, lmData); err != nil {
			return nil, fmt.Errorf("layer mask data: %w", err)
		}
		if err := parseLayerSection(lmData, psd); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no layer data found in PSD")
	}

	if len(psd.Layers) == 0 {
		return nil, fmt.Errorf("no layers found in PSD")
	}

	return psd, nil
}

// parseLayerSection parses the layer & mask information block.
func parseLayerSection(data []byte, psd *psdFile) error {
	r := &sliceReader{data: data}

	// ---------- layer info ----------
	layerInfoLen := int(r.u32())
	if layerInfoLen == 0 {
		return nil
	}

	end := r.pos + layerInfoLen

	// layer count (signed: negative means first layer carries transparency)
	lc := int16(r.u16())
	layerCount := int(lc)
	if layerCount < 0 {
		layerCount = -layerCount
	}
	if layerCount == 0 {
		return nil
	}

	type chanInfo struct {
		id     int16
		length uint32
	}
	allChanInfo := make([][]chanInfo, layerCount)

	for i := 0; i < layerCount; i++ {
		l := &psdLayer{}
		l.Top = int32(r.i32())
		l.Left = int32(r.i32())
		l.Bottom = int32(r.i32())
		l.Right = int32(r.i32())

		chCount := int(r.u16())
		ci := make([]chanInfo, chCount)
		for j := 0; j < chCount; j++ {
			ci[j].id = int16(r.i16())
			ci[j].length = r.u32()
		}
		allChanInfo[i] = ci

		// blend mode signature "8BIM"
		_ = r.bytes(4)
		// blend mode key
		blendKey := string(r.bytes(4))
		opacity := r.u8()
		clipping := r.u8()
		flags := r.u8()
		// bit 1 = 0 → visible
		visible := (flags & 0x02) == 0
		// filler
		_ = r.u8()

		// extra data length
		extraLen := int(r.i32())
		extraStart := r.pos

		// layer mask data (length + data)
		maskLen := int(r.i32())
		if maskLen > 0 {
			r.skip(maskLen)
		}
		// layer blending ranges (length + data)
		blendLen := int(r.i32())
		if blendLen > 0 {
			r.skip(blendLen)
		}
		// layer name: Pascal string padded to 4 bytes
		nameLen := int(r.u8())
		nameBytes := r.bytes(nameLen)
		l.Name = string(nameBytes)
		pad := (nameLen + 1) % 4
		if pad > 0 {
			r.skip(4 - pad)
		}

		// skip remaining extra data (ancestors, etc.)
		remaining := extraLen - (r.pos - extraStart)
		if remaining > 0 {
			r.skip(remaining)
		}

		l.BlendMode = blendKey
		l.Opacity = opacity
		l.Clipping = clipping
		l.Visible = visible
		psd.Layers = append(psd.Layers, l)
	}

	// ---------- channel image data for each layer ----------
	// Ensure we are at the right position
	if r.pos < end {
		// There may be padding or additional data between layer records and channel data
		// The channel data starts right after the layer records
	}

	// For safety, seek to the start of channel data area explicitly
	// The channel data begins immediately after the layer records (at the position we're at)
	// But we need to account for the fact that r.pos may be before end

	for i, layer := range psd.Layers {
		ci := allChanInfo[i]
		layerW := int(layer.Right - layer.Left)
		layerH := int(layer.Bottom - layer.Top)
		if layerW <= 0 || layerH <= 0 {
			continue
		}

		var red, green, blue, alpha []byte
		hasAlpha := false

		for _, ch := range ci {
			if ch.length == 0 {
				// data stored in composite section — skip
				continue
			}
			compression := int(r.u16()) // 0=Raw, 1=RLE
			var chData []byte
			if ch.length > 2 {
				if compression == 1 {
					// RLE: read per-row byte counts, then compressed data
					rowCounts := make([]uint16, layerH)
					for row := 0; row < layerH; row++ {
						rowCounts[row] = r.u16()
					}
					// read and decompress each row
					chData = make([]byte, layerH*layerW)
					for row := 0; row < layerH; row++ {
						rowComp := r.bytes(int(rowCounts[row]))
						decompressed := dePackBitsRow(rowComp, layerW)
						copy(chData[row*layerW:], decompressed)
					}
				} else {
					// Raw
					chData = r.bytes(layerW * layerH)
				}
			}

			switch ch.id {
			case 0:
				red = chData
			case 1:
				green = chData
			case 2:
				blue = chData
			case -1:
				alpha = chData
				hasAlpha = true
			}
		}

		// Construct NRGBA image from channel data
		img := image.NewNRGBA(image.Rect(0, 0, layerW, layerH))
		for y := 0; y < layerH; y++ {
			for x := 0; x < layerW; x++ {
				idx := y*layerW + x
				rv := byte(0)
				gv := byte(0)
				bv := byte(0)
				av := byte(255)
				if red != nil {
					rv = red[idx]
				}
				if green != nil {
					gv = green[idx]
				}
				if blue != nil {
					bv = blue[idx]
				}
				if hasAlpha && alpha != nil {
					av = alpha[idx]
				}
				img.SetNRGBA(x, y, color.NRGBA{R: rv, G: gv, B: bv, A: av})
			}
		}
		layer.Data = img
	}

	return nil
}

// ---------------------------------------------------------------------------
// PSD writer
// ---------------------------------------------------------------------------

// compressedChannel stores pre-compressed channel data for a single channel.
type compressedChannel struct {
	id          int16
	compression uint16 // 1 = RLE
	rowSizes    []uint16
	data        []byte // concatenated compressed row data
}

// layerWriteData holds pre-computed write information for a single layer.
type layerWriteData struct {
	channels   []compressedChannel
	extraLen   int // size of extra data section (mask + blending ranges + name + padding)
	namePadded int // padded name bytes including length byte
}

func writePSD(path string, psd *psdFile) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output psd: %w", err)
	}
	defer f.Close()

	w := &psdWriter{w: f}
	return w.write(psd)
}

type psdWriter struct {
	w io.Writer
}

func (w *psdWriter) write(psd *psdFile) error {
	// Pre-compress all layer channel data
	type cc struct {
		ch   compressedChannel
		size int // total bytes for this channel (compression + row sizes + data)
	}
	type layerPrep struct {
		channels []cc
	}

	prep := make([]layerPrep, len(psd.Layers))

	for li, layer := range psd.Layers {
		layerW := int(layer.Right - layer.Left)
		layerH := int(layer.Bottom - layer.Top)

		// Determine channel IDs: if layer has no alpha, use [0,1,2]; else [-1,0,1,2]
		hasAlpha := layerHasAlpha(layer)
		var chIDs []int16
		if hasAlpha {
			chIDs = []int16{-1, 0, 1, 2}
		} else {
			chIDs = []int16{0, 1, 2}
		}

		for _, cid := range chIDs {
			// Extract channel bytes from NRGBA data
			rawCh := make([]byte, layerW*layerH)
			for y := 0; y < layerH; y++ {
				for x := 0; x < layerW; x++ {
					c := layer.Data.NRGBAAt(x, y)
					switch cid {
					case 0:
						rawCh[y*layerW+x] = c.R
					case 1:
						rawCh[y*layerW+x] = c.G
					case 2:
						rawCh[y*layerW+x] = c.B
					case -1:
						rawCh[y*layerW+x] = c.A
					}
				}
			}

			// RLE compress each row
			var compressed []byte
			rowSizes := make([]uint16, layerH)
			for row := 0; row < layerH; row++ {
				rowData := rawCh[row*layerW : (row+1)*layerW]
				packed := packBitsRow(rowData)
				rowSizes[row] = uint16(len(packed))
				compressed = append(compressed, packed...)
			}

			chSize := 2 + layerH*2 + len(compressed) // compression + row sizes + data

			prep[li].channels = append(prep[li].channels, cc{
				ch: compressedChannel{
					id:          cid,
					compression: 1,
					rowSizes:    rowSizes,
					data:        compressed,
				},
				size: chSize,
			})
		}
	}

	// Build composite image from visible layers
	compImage := compositeLayers(psd.Layers, psd.Width, psd.Height)
	compChannels := psd.Channels
	if compChannels != 3 && compChannels != 4 {
		compChannels = 4
	}

	// Compute sizes for layer info section
	layerInfoLen := 2 // layer count
	for li, layer := range psd.Layers {
		// Layer record: 4×4 bounds + 2 chCount + (4+4)×n channels + 4 sig + 4 blend + 1+1+1+1
		recSize := 16 + 2 + len(prep[li].channels)*6 + 4 + 4 + 4
		// extra data: extraLen(4) + mask(4+0) + blending(4+0) + name (1 + len + padding)
		nameLen := len(layer.Name)
		namePadded := nameLen + 1 // +1 for length byte
		padding := (4 - namePadded%4) % 4
		extraDataSize := 8 + namePadded + padding // mask (4+0) + blending(4+0) + name
		recSize += 4 + extraDataSize                   // extraLen field + extra data itself
		layerInfoLen += recSize
	}
	// Add channel image data size
	for _, lp := range prep {
		for _, c := range lp.channels {
			layerInfoLen += c.size
		}
	}

	// Layer & mask section total length
	layerMaskLen := 4 + layerInfoLen // length fields: layer info length(4) + layer info data
	layerMaskLen += 4                // global layer mask length (= 0)

	// Extract raw composite image data for each channel
	compRaw := make([][]byte, compChannels)
	for ci := 0; ci < compChannels; ci++ {
		rawCh := make([]byte, psd.Width*psd.Height)
		for y := 0; y < psd.Height; y++ {
			for x := 0; x < psd.Width; x++ {
				c := compImage.NRGBAAt(x, y)
				switch ci {
				case 0:
					rawCh[y*psd.Width+x] = c.R
				case 1:
					rawCh[y*psd.Width+x] = c.G
				case 2:
					rawCh[y*psd.Width+x] = c.B
				case 3:
					rawCh[y*psd.Width+x] = c.A
				}
			}
		}
		compRaw[ci] = rawCh
	}

	// ---------- Write header ----------
	put16 := func(v uint16) { binary.Write(w.w, binary.BigEndian, v) }
	put32 := func(v uint32) { binary.Write(w.w, binary.BigEndian, v) }

	w.w.Write([]byte("8BPS"))
	put16(1)       // version
	w.w.Write([]byte{0, 0, 0, 0, 0, 0}) // reserved
	put16(uint16(compChannels))
	put32(uint32(psd.Height))
	put32(uint32(psd.Width))
	put16(uint16(psd.Depth))
	put16(uint16(psd.ColorMode))

	// Color mode data: length 0
	put32(0)

	// ---------- Image Resources ----------
	// Write minimal required resources for compatibility (resolution info + print flags)
	imgRes := buildImageResources()
	put32(uint32(len(imgRes)))
	w.w.Write(imgRes)

	// ---------- Layer & Mask Information ----------
	put32(uint32(layerMaskLen))

	// Layer info length (includes layer count + records + channel data)
	put32(uint32(layerInfoLen))

	// Layer count (always positive for maximum compatibility)
	put16(uint16(len(psd.Layers)))

	// Layer records
	for li, layer := range psd.Layers {
		channels := prep[li].channels
		put32(uint32(layer.Top))
		put32(uint32(layer.Left))
		put32(uint32(layer.Bottom))
		put32(uint32(layer.Right))
		put16(uint16(len(channels)))
		for _, c := range channels {
			put16(uint16(c.ch.id))
			// Channel data length includes compression(2) + rowSizes(height*2) + compressed data
			chLen := uint32(2 + len(c.ch.rowSizes)*2 + len(c.ch.data))
			put32(chLen)
		}

		// Blend mode
		w.w.Write([]byte("8BIM"))
		blend := []byte(layer.BlendMode)
		if len(blend) < 4 {
			blend = append(blend, ' ', ' ', ' ', ' ')
		}
		w.w.Write(blend[:4])

		w.w.Write([]byte{layer.Opacity, layer.Clipping})

		// Flags: bit 1 = 0 for visible
		flags := byte(0)
		if !layer.Visible {
			flags |= 0x02
		}
		w.w.Write([]byte{flags, 0}) // flags + filler

		// Extra data: mask (4+0) + blending (4+0) + name
		nameBytes := []byte(layer.Name)
		nameLen := len(nameBytes)
		namePadded := nameLen + 1 // +1 for length byte
		padding := (4 - namePadded%4) % 4
		extraDataSize := 8 + namePadded + padding
		put32(uint32(extraDataSize))

		// Layer mask: length 0
		put32(0)
		// Blending ranges: length 0
		put32(0)
		// Name: Pascal string
		w.w.Write([]byte{byte(nameLen)})
		w.w.Write(nameBytes)
		// Padding
		for p := 0; p < padding; p++ {
			w.w.Write([]byte{0})
		}
	}

	// Channel image data for each layer
	for li := range psd.Layers {
		for _, c := range prep[li].channels {
			put16(c.ch.compression) // = 1 (RLE)
			for _, sz := range c.ch.rowSizes {
				put16(sz)
			}
			w.w.Write(c.ch.data)
		}
	}

	// Global layer mask: length 0
	put32(0)

	// ---------- Composite Image Data ----------
	// Compression = 0 (Raw) for maximum compatibility
	put16(0)
	for ci := 0; ci < compChannels; ci++ {
		w.w.Write(compRaw[ci])
	}

	return nil
}

// ---------------------------------------------------------------------------
// Alpha detection
// ---------------------------------------------------------------------------

func layerHasAlpha(l *psdLayer) bool {
	if l.Data == nil {
		return false
	}
	b := l.Data.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if l.Data.NRGBAAt(x, y).A < 255 {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Composite rendering (alpha blending, normal mode)
// ---------------------------------------------------------------------------

func compositeLayers(layers []*psdLayer, w, h int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	// dst starts as transparent black (zero values)

	for _, layer := range layers {
		if !layer.Visible || layer.Data == nil {
			continue
		}
		lw := int(layer.Right - layer.Left)
		lh := int(layer.Bottom - layer.Top)
		offX := int(layer.Left)
		offY := int(layer.Top)
		opacity := float64(layer.Opacity) / 255.0

		for y := 0; y < lh; y++ {
			for x := 0; x < lw; x++ {
				dx := offX + x
				dy := offY + y
				if dx < 0 || dx >= w || dy < 0 || dy >= h {
					continue
				}
				src := layer.Data.NRGBAAt(x, y)
				srcA := float64(src.A) * opacity / 255.0
				if srcA <= 0 {
					continue
				}
				dstC := dst.NRGBAAt(dx, dy)
				dstA := float64(dstC.A) / 255.0

				// Alpha blending: src OVER dst
				outA := srcA + dstA*(1-srcA)
				if outA <= 0 {
					continue
				}
				outR := (float64(src.R)*srcA + float64(dstC.R)*dstA*(1-srcA)) / outA
				outG := (float64(src.G)*srcA + float64(dstC.G)*dstA*(1-srcA)) / outA
				outB := (float64(src.B)*srcA + float64(dstC.B)*dstA*(1-srcA)) / outA

				dst.SetNRGBA(dx, dy, color.NRGBA{
					R: uint8(clamp(outR)),
					G: uint8(clamp(outG)),
					B: uint8(clamp(outB)),
					A: uint8(clamp(outA * 255)),
				})
			}
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// PackBits RLE (as used by PSD)
// ---------------------------------------------------------------------------

// dePackBitsRow decompresses a PackBits-encoded row into expectedLen bytes.
func dePackBitsRow(data []byte, expectedLen int) []byte {
	res := make([]byte, 0, expectedLen)
	for i := 0; i < len(data); {
		header := int8(data[i])
		i++
		if header >= 0 {
			n := int(header) + 1
			if i+n > len(data) {
				n = len(data) - i
			}
			res = append(res, data[i:i+n]...)
			i += n
		} else if header > -128 {
			n := int(-header) + 1
			if i >= len(data) {
				break
			}
			b := data[i]
			i++
			for j := 0; j < n; j++ {
				res = append(res, b)
			}
		}
		// header == -128: no-op
	}
	return res
}

// packBitsRow encodes a single row of data using PackBits RLE.
func packBitsRow(data []byte) []byte {
	var result []byte
	n := len(data)
	i := 0
	for i < n {
		// Check if we have a repeat run (3+ identical bytes)
		if i+2 < n && data[i] == data[i+1] && data[i+1] == data[i+2] {
			repeatVal := data[i]
			runLen := 1
			for i+runLen < n && data[i+runLen] == repeatVal && runLen < 128 {
				runLen++
			}
			header := byte(-(runLen - 1)) // int8 → byte
			result = append(result, header, repeatVal)
			i += runLen
		} else {
			// Literal run (max 128 bytes)
			start := i
			end := i + 1
			for end < n && end-start < 128 {
				// Stop if the next 3 bytes are identical (better as repeat run)
				if end+2 < n && data[end] == data[end+1] && data[end+1] == data[end+2] {
					break
				}
				end++
			}
			literalLen := end - start
			header := byte(literalLen - 1)
			result = append(result, header)
			result = append(result, data[start:end]...)
			i = end
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Image resources
// ---------------------------------------------------------------------------

func buildImageResources() []byte {
	var buf []byte

	// Resource 0x03ED: Print Resolution Info (16 bytes)
	// Horizontal resolution: 72.0 ppi as 16.16 fixed point
	buf = appendResource(buf, 0x03ED, []byte{
		0x00, 0x48, 0x00, 0x00, // 72.0 ppi horizontal
		0x01,                     // unit: pixels per inch
		0x01,                     // width unit: inches
		0x00, 0x48, 0x00, 0x00, // 72.0 ppi vertical
		0x01, // unit: pixels per inch
		0x01, // height unit: inches
	})

	// Resource 0x0302: Print Flags (1 byte: all zeros = no flags)
	buf = appendResource(buf, 0x0302, []byte{0x00})

	return buf
}

func appendResource(buf []byte, id uint16, data []byte) []byte {
	// Signature "8BIM"
	buf = append(buf, '8', 'B', 'I', 'M')
	// Resource ID
	var idBuf [2]byte
	binary.BigEndian.PutUint16(idBuf[:], id)
	buf = append(buf, idBuf[:]...)
	// Name: Pascal string, padded to even (empty = 0x00 + 0x00)
	buf = append(buf, 0x00, 0x00)
	// Data length
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	buf = append(buf, lenBuf[:]...)
	// Data
	buf = append(buf, data...)
	return buf
}

// ---------------------------------------------------------------------------
// sliceReader — helper to read from a []byte at a position
// ---------------------------------------------------------------------------

type sliceReader struct {
	data []byte
	pos  int
}

func (r *sliceReader) u8() byte {
	b := r.data[r.pos]
	r.pos++
	return b
}

func (r *sliceReader) u16() uint16 {
	v := binary.BigEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v
}

func (r *sliceReader) i16() int16 {
	return int16(r.u16())
}

func (r *sliceReader) u32() uint32 {
	v := binary.BigEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v
}

func (r *sliceReader) i32() int32 {
	return int32(r.u32())
}

func (r *sliceReader) bytes(n int) []byte {
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *sliceReader) skip(n int) {
	r.pos += n
}
