package agent

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/ozodmeofficial/smarteye/internal/protocol"
	"github.com/ozodmeofficial/smarteye/internal/screen"
)

// streamer captures the screen and emits frames to the server. It supports two
// independent cadences at once: a low-rate thumbnail preview (for the grid tile)
// and a high-rate full-quality stream (when the operator opens this machine).
type streamer struct {
	cap  screen.Capturer
	send func(*protocol.Envelope) error

	mu          sync.Mutex
	monitor     int
	thumbOn     bool
	thumbFPS    int
	thumbW      int
	thumbQ      int
	streamOn    bool
	streamFPS   int
	streamQ     int
	seq         uint64

	wake chan struct{}
}

func newStreamer(cap screen.Capturer, send func(*protocol.Envelope) error) *streamer {
	return &streamer{
		cap:      cap,
		send:     send,
		thumbFPS: 1,
		thumbW:   360,
		thumbQ:   55,
		wake:     make(chan struct{}, 1),
	}
}

func (s *streamer) setThumb(req protocol.ThumbRequest) {
	s.mu.Lock()
	s.thumbOn = req.Enabled
	if req.FPS > 0 {
		s.thumbFPS = req.FPS
	}
	if req.MaxWidth > 0 {
		s.thumbW = req.MaxWidth
	}
	if req.Quality > 0 {
		s.thumbQ = req.Quality
	}
	s.mu.Unlock()
	s.poke()
}

func (s *streamer) setStream(on bool, mon, fps, quality int) {
	s.mu.Lock()
	s.streamOn = on
	if on {
		s.monitor = mon
		s.streamFPS = fps
		s.streamQ = quality
		if s.streamFPS <= 0 {
			s.streamFPS = 20
		}
		if s.streamQ <= 0 {
			s.streamQ = 70
		}
	}
	s.mu.Unlock()
	s.poke()
}

func (s *streamer) selectMonitor(mon int) {
	s.mu.Lock()
	s.monitor = mon
	s.mu.Unlock()
	s.poke()
}

func (s *streamer) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// run drives the capture loop until ctx is cancelled. It computes the next
// deadline from whichever cadence (thumb/stream) is active and sleeps until
// then, waking early when settings change.
func (s *streamer) run(ctx context.Context) {
	var lastThumb, lastStream time.Time
	for {
		s.mu.Lock()
		thumbOn, streamOn := s.thumbOn, s.streamOn
		thumbFPS, streamFPS := s.thumbFPS, s.streamFPS
		s.mu.Unlock()

		if !thumbOn && !streamOn {
			// Idle: wait for a wake or cancellation. This is the "nearly
			// asleep" state that keeps client CPU at ~0 when unobserved.
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
				continue
			}
		}

		now := time.Now()
		var sleep time.Duration = time.Hour

		if streamOn {
			interval := time.Second / time.Duration(max(streamFPS, 1))
			if now.Sub(lastStream) >= interval {
				s.captureAndSend(false)
				lastStream = now
			}
			if d := interval - time.Since(lastStream); d < sleep && d > 0 {
				sleep = d
			} else if sleep > interval {
				sleep = interval
			}
		}
		if thumbOn {
			interval := time.Second / time.Duration(max(thumbFPS, 1))
			if now.Sub(lastThumb) >= interval {
				// Skip a redundant thumbnail if a full stream is already live;
				// the operator sees the big view anyway.
				if !streamOn {
					s.captureAndSend(true)
				}
				lastThumb = now
			}
			if d := interval - time.Since(lastThumb); d < sleep && d > 0 {
				sleep = d
			}
		}

		if sleep <= 0 {
			sleep = 10 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		case <-s.wake:
		}
	}
}

// captureAndSend grabs one frame and emits it as a thumb or stream frame.
func (s *streamer) captureAndSend(thumb bool) {
	s.mu.Lock()
	mon := s.monitor
	var q, w int
	var typ protocol.MessageType
	if thumb {
		q, w, typ = s.thumbQ, s.thumbW, protocol.TypeThumbFrame
	} else {
		q, w, typ = s.streamQ, 0, protocol.TypeStreamFrame
	}
	s.seq++
	seq := s.seq
	s.mu.Unlock()

	img, err := s.cap.Capture(mon)
	if err != nil {
		return
	}
	data, fw, fh, err := screen.EncodeJPEG(img, q, w)
	if err != nil {
		return
	}
	frame := protocol.Frame{
		Monitor: mon,
		Width:   fw,
		Height:  fh,
		Format:  "jpeg",
		Full:    true,
		Data:    base64.StdEncoding.EncodeToString(data),
		Seq:     seq,
	}
	if env, err := protocol.NewEnvelope(typ, frame); err == nil {
		_ = s.send(env)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
