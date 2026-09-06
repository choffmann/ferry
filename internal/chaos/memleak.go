package chaos

import (
	"context"
	"os"
	"sync"
	"time"
)

const (
	bytesPerMB = 1 << 20

	msPerMinute = int64(time.Minute / time.Millisecond)
)

var pageSize = os.Getpagesize()

// Leaker holds on to memory on purpose. It reads the switch on every tick
// instead of being started and stopped, so a reset releases what it holds.
type Leaker struct {
	store    Store
	interval time.Duration

	mu     sync.Mutex
	blocks [][]byte
	// owed carries the fraction of a megabyte a tick did not allocate yet, in
	// megabyte-milliseconds, so the rate stays exact instead of being truncated.
	owed int64
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
		l.owed = 0
		return
	}

	l.owed += int64(s.Resources.MemoryLeakMBPerMin) * l.interval.Milliseconds()
	perTick := int(l.owed / msPerMinute)
	l.owed -= int64(perTick) * msPerMinute
	for i := 0; i < perTick; i++ {
		block := make([]byte, bytesPerMB)
		// Go serves a block this size from freshly mapped zero pages and skips
		// zeroing them, so every page has to be written or the memory counts
		// against the heap but never becomes resident. Writing only the first
		// byte delivered 2.4% of the requested megabytes.
		for off := 0; off < len(block); off += pageSize {
			block[off] = 1
		}
		l.blocks = append(l.blocks, block)
	}
}
