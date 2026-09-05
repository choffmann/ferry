package chaos

import (
	"context"
	"sync"
	"time"
)

const bytesPerMB = 1 << 20

// Leaker holds on to memory on purpose. It reads the switch on every tick
// instead of being started and stopped, so a reset releases what it holds.
type Leaker struct {
	store    Store
	interval time.Duration

	mu     sync.Mutex
	blocks [][]byte
}

func NewLeaker(store Store, interval time.Duration) *Leaker {
	return &Leaker{store: store, interval: interval}
}

func (l *Leaker) AllocatedMB() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.blocks)
}

func (l *Leaker) Run(ctx context.Context) {
	t := time.NewTicker(l.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.tick(ctx)
		}
	}
}

func (l *Leaker) tick(ctx context.Context) {
	s, err := l.store.Get(ctx)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if s.Resources.MemoryLeakMBPerMin <= 0 {
		l.blocks = nil
		return
	}

	perTick := int(float64(s.Resources.MemoryLeakMBPerMin) * l.interval.Seconds() / 60.0)
	if perTick < 1 {
		perTick = 1
	}
	for i := 0; i < perTick; i++ {
		block := make([]byte, bytesPerMB)
		// Touching a byte keeps the page from staying untouched and unaccounted.
		block[0] = 1
		l.blocks = append(l.blocks, block)
	}
}
