package upscale

import (
	"fmt"
	"slices"
)

type Engine interface {
	Name() string
	Available() (bool, string)
	Upscale(req Request) (Result, error)
}

type Manager struct {
	engines []Engine
}

func NewManager() *Manager {
	return &Manager{
		engines: []Engine{
			NewAnime4KCPPEngine(),
			NewRealSREngine(),
			NewWaifu2xEngine(),
			NewRealCUGANEngine(),
			NewRealESRGANEngine(),
			NewBuiltinEngine(),
		},
	}
}

func (m *Manager) Available() []EngineInfo {
	out := make([]EngineInfo, 0, len(m.engines))
	for _, engine := range m.engines {
		ok, detail := engine.Available()
		status := "unavailable"
		if ok {
			status = "available"
		}
		if detail != "" {
			status = fmt.Sprintf("%s (%s)", status, detail)
		}
		out = append(out, EngineInfo{Name: engine.Name(), Status: status})
	}
	return out
}

func (m *Manager) Upscale(req Request) (Result, error) {
	if req.Engine == "" || req.Engine == "auto" {
		for _, engine := range m.engines {
			if ok, _ := engine.Available(); ok {
				return engine.Upscale(req)
			}
		}
		return Result{}, fmt.Errorf("no available engine found")
	}

	names := make([]string, 0, len(m.engines))
	for _, engine := range m.engines {
		names = append(names, engine.Name())
		if engine.Name() == req.Engine {
			ok, detail := engine.Available()
			if !ok {
				return Result{}, fmt.Errorf("%s is unavailable: %s", req.Engine, detail)
			}
			return engine.Upscale(req)
		}
	}

	slices.Sort(names)
	return Result{}, fmt.Errorf("unknown engine %q, supported: %v", req.Engine, names)
}

func (m *Manager) IsAvailable(name string) (bool, string) {
	for _, engine := range m.engines {
		if engine.Name() == name {
			return engine.Available()
		}
	}
	return false, "unknown engine"
}
