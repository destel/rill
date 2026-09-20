package rill

import (
	"fmt"
	"strings"
	"testing"
)

func TestWithContext(t *testing.T) {
	t.Run("exactly one sink", func(t *testing.T) {
		_, opt := WithContext(t.Context())

		// the first sink is fine
		_ = Err(FromSlice([]int{}, nil), opt)

		defer func() {
			msg := fmt.Sprint(recover())
			if !strings.Contains(msg, "exactly one sink") {
				t.Fatalf("expected the single-sink panic, got: %v", msg)
			}
		}()

		// the second one panics
		_ = Err(FromSlice([]int{}, nil), opt)
	})
}
