package store

import (
	"testing"
	"time"
)

func TestMemoryStoreSatisfiesTheContract(t *testing.T) {
	runRepositoryContract(t, func(now time.Time) Repository {
		return NewMemoryStore(now)
	})
}
