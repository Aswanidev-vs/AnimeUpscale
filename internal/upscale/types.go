package upscale

type Request struct {
	Input     string
	Output    string
	Engine    string
	Scale     int
	TargetW   int
	TargetH   int
	Target    string
	Noise     int
	Format    string
	TileSize  int
	GPUID     string
	ModelPath string
	ModelName string
	Threads   string
	TTA       bool
	Sharpen   float64
	Grayscale bool
	LayerMode string // psd layer mode: "visible" (default), "all"
}

type Result struct {
	Engine       string
	Output       string
	InputWidth   int
	InputHeight  int
	OutputWidth  int
	OutputHeight int
	Note         string
	LayerCount   int // number of layers processed (for PSD input)
}

type EngineInfo struct {
	Name   string
	Status string
}
