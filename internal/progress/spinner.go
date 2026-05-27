package progress

import (
	"fmt"
	"sync"
	"time"
)

type Spinner struct {
	stopChan chan struct{}
	wg       sync.WaitGroup
	message  string
}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		stopChan: make(chan struct{}),
		message:  message,
	}
}

func (s *Spinner) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		width := 20
		step := 0
		total := width + 4 // extra frames to hold at full before resetting
		for {
			select {
			case <-s.stopChan:
				// Print final completed bar
				fmt.Printf("\r[%s] %s... Done!\n", repeatRune('█', width), s.message)
				return
			default:
				filled := step
				if filled > width {
					filled = width
				}
				empty := width - filled
				fmt.Printf("\r[%s%s] %s...", repeatRune('█', filled), repeatRune('░', empty), s.message)
				step++
				if step >= total {
					step = 0
				}
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func repeatRune(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]rune, n)
	for i := range buf {
		buf[i] = r
	}
	return string(buf)
}
