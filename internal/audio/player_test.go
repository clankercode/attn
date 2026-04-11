package audio

import (
	"testing"

	"github.com/faiface/beep"
)

type finiteStreamer struct {
	remaining int
}

func (s *finiteStreamer) Stream(samples [][2]float64) (int, bool) {
	if s.remaining <= 0 {
		return 0, false
	}
	if len(samples) > s.remaining {
		n := s.remaining
		s.remaining = 0
		return n, true
	}
	s.remaining -= len(samples)
	return len(samples), true
}

func (s *finiteStreamer) Err() error { return nil }

func TestStreamerWithDoneClosesAfterSourceEnds(t *testing.T) {
	done := make(chan struct{})
	streamer := streamerWithDone(&finiteStreamer{remaining: 2}, done)
	samples := make([][2]float64, 4)

	if _, ok := streamer.Stream(samples[:1]); !ok {
		t.Fatal("expected streamer to remain active before source ends")
	}
	select {
	case <-done:
		t.Fatal("done closed before source ended")
	default:
	}

	streamer.Stream(samples)
	streamer.Stream(samples)

	select {
	case <-done:
	default:
		t.Fatal("expected done to close after source ended")
	}
}

var _ beep.Streamer = (*finiteStreamer)(nil)
