package store

import (
	"testing"
	"time"
)

func TestFakeSatisfiesTheContract(t *testing.T) {
	runRepositoryContract(t, func(seed time.Time, now func() time.Time) Repository {
		return newFake(seed, now)
	})
}
