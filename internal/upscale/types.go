package upscale

type Request struct {
	Input     string
	Output    string
	Engine    string
	Scale     int
	TargetW   int
	TargetH   int
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
}

type Result struct {
	Engine       string
	Output       string
	InputWidth   int
	InputHeight  int
	OutputWidth  int
	OutputHeight int
	Note         string
}

type EngineInfo struct {
	Name   string
	Status string
}
