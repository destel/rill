package rill

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestDrain(t *testing.T) {
	// real tests are in another package
	Drain[int](th.FromRange(0, 10))
	Discard[int](th.FromRange(0, 10))
	DrainNB[int](th.FromRange(0, 10))
}

func TestBuffer(t *testing.T) {
	// real tests are in another package
	Buffer[int](th.FromRange(0, 10), 5)
}
