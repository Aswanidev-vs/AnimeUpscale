package upscale

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
)

type builtinEngine struct{}

func NewBuiltinEngine() Engine {
	return builtinEngine{}
}

func (builtinEngine) Name() string {
	return "builtin"
}

func (builtinEngine) Available() (bool, string) {
	return true, "pure go fallback"
}

func (builtinEngine) Upscale(req Request) (Result, error) {
	src, bounds, err := decodeImage(req.Input)
	if err != nil {
		return Result{}, err
	}

	work := src
	if req.Grayscale {
		work = grayscale(work)
	}

	dst := scaleImage(work, req.Scale, req.TargetW, req.TargetH)
	dst = animeEnhance(dst, req.Sharpen)

	if err := encodeImage(req.Output, req.Format, dst); err != nil {
		return Result{}, err
	}

	return Result{
		Engine:       "builtin",
		Output:       req.Output,
		InputWidth:   bounds.Dx(),
		InputHeight:  bounds.Dy(),
		OutputWidth:  dst.Bounds().Dx(),
		OutputHeight: dst.Bounds().Dy(),
		Note:         "pure Go upscale with Anime4K-style line enhancement and local contrast boost",
	}, nil
}

func scaleImage(src *image.NRGBA, scale, targetW, targetH int) *image.NRGBA {
	if targetW > 0 && targetH > 0 {
		return BilinearResize(src, targetW, targetH)
	}
	return bilinearScale(src, scale)
}

func decodeImage(path string) (*image.NRGBA, image.Rectangle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("decode input: %w", err)
	}
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return dst, bounds, nil
}

func encodeImage(path, format string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}

	var encodeErr error
	switch format {
	case "jpg", "jpeg":
		encodeErr = jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
	default:
		encodeErr = png.Encode(f, img)
	}

	f.Close()
	if encodeErr != nil {
		_ = os.Remove(path) // prevent corrupted file
		return fmt.Errorf("encode output: %w", encodeErr)
	}
	return nil
}

func bilinearScale(src *image.NRGBA, scale int) *image.NRGBA {
	if scale <= 1 {
		return BilinearResize(src, src.Bounds().Dx(), src.Bounds().Dy())
	}
	srcB := src.Bounds()
	return BilinearResize(src, srcB.Dx()*scale, srcB.Dy()*scale)
}

func BilinearResize(src *image.NRGBA, dstW, dstH int) *image.NRGBA {
	if dstW <= 0 || dstH <= 0 {
		dstW = src.Bounds().Dx()
		dstH = src.Bounds().Dy()
	}
	if dstW == src.Bounds().Dx() && dstH == src.Bounds().Dy() {
		clone := image.NewNRGBA(src.Bounds())
		copy(clone.Pix, src.Pix)
		return clone
	}
	srcB := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))

	for y := 0; y < dstH; y++ {
		fy := float64(y) * float64(srcB.Dy()-1) / float64(max(dstH-1, 1))
		y0 := int(math.Floor(fy))
		y1 := min(y0+1, srcB.Dy()-1)
		wy := fy - float64(y0)
		for x := 0; x < dstW; x++ {
			fx := float64(x) * float64(srcB.Dx()-1) / float64(max(dstW-1, 1))
			x0 := int(math.Floor(fx))
			x1 := min(x0+1, srcB.Dx()-1)
			wx := fx - float64(x0)
			c00 := src.NRGBAAt(x0, y0)
			c10 := src.NRGBAAt(x1, y0)
			c01 := src.NRGBAAt(x0, y1)
			c11 := src.NRGBAAt(x1, y1)
			dst.SetNRGBA(x, y, interpolate4(c00, c10, c01, c11, wx, wy))
		}
	}

	return dst
}

func animeEnhance(src *image.NRGBA, amount float64) *image.NRGBA {
	smoothed := boxBlur(src)
	enhanced := image.NewNRGBA(src.Bounds())
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			orig := src.NRGBAAt(x, y)
			blur := smoothed.NRGBAAt(x, y)
			gx, gy := gradientAt(src, x, y)
			edge := minFloat(1, math.Sqrt(gx*gx+gy*gy)/96.0)

			r := enhanceChannel(orig.R, blur.R, edge, amount)
			g := enhanceChannel(orig.G, blur.G, edge, amount)
			b := enhanceChannel(orig.B, blur.B, edge, amount)

			enhanced.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: orig.A})
		}
	}
	return enhanced
}

func interpolate4(c00, c10, c01, c11 color.NRGBA, wx, wy float64) color.NRGBA {
	blend := func(a, b, c, d uint8) uint8 {
		top := float64(a)*(1-wx) + float64(b)*wx
		bot := float64(c)*(1-wx) + float64(d)*wx
		out := top*(1-wy) + bot*wy
		return uint8(clamp(out))
	}
	return color.NRGBA{
		R: blend(c00.R, c10.R, c01.R, c11.R),
		G: blend(c00.G, c10.G, c01.G, c11.G),
		B: blend(c00.B, c10.B, c01.B, c11.B),
		A: blend(c00.A, c10.A, c01.A, c11.A),
	}
}

func grayscale(src *image.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			c := src.NRGBAAt(x, y)
			l := uint8((299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000)
			dst.SetNRGBA(x, y, color.NRGBA{R: l, G: l, B: l, A: c.A})
		}
	}
	return dst
}

func boxBlur(src *image.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			var r, g, b, a, count int
			for ky := max(y-1, 0); ky <= min(y+1, src.Bounds().Dy()-1); ky++ {
				for kx := max(x-1, 0); kx <= min(x+1, src.Bounds().Dx()-1); kx++ {
					c := src.NRGBAAt(kx, ky)
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a += int(c.A)
					count++
				}
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / count),
				G: uint8(g / count),
				B: uint8(b / count),
				A: uint8(a / count),
			})
		}
	}
	return dst
}

func gradientAt(src *image.NRGBA, x, y int) (float64, float64) {
	l := luminance(src.NRGBAAt(max(x-1, 0), y))
	r := luminance(src.NRGBAAt(min(x+1, src.Bounds().Dx()-1), y))
	u := luminance(src.NRGBAAt(x, max(y-1, 0)))
	d := luminance(src.NRGBAAt(x, min(y+1, src.Bounds().Dy()-1)))
	return r - l, d - u
}

func luminance(c color.NRGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}

func enhanceChannel(orig, blur uint8, edge, amount float64) uint8 {
	base := float64(orig)
	soft := float64(blur)
	highpass := base - soft
	lineBoost := highpass * (0.45 + amount*1.8) * edge
	flatRecovery := (soft - base) * 0.08 * (1 - edge)
	return uint8(clamp(base + lineBoost + flatRecovery))
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return math.Round(v)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

