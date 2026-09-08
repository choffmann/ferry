package store

import (
	"testing"
	"time"
)

func TestMemoryStoreSatisfiesTheContract(t *testing.T) {
	runRepositoryContract(t, func(seed time.Time, now func() time.Time) Repository {
		return newMemoryStore(seed, now)
	})
}
