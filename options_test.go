package rill

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestSinkOption(t *testing.T) {
	th.RunSynctest(t, "nil options are ignored", func(t *testing.T) {
		var appliedCnt atomic.Int32
		opt := sinkOptionFunc(func(options *sinkOptions) {
			appliedCnt.Add(1)
		})

		Discard(th.FromSlice([]int{}), opt, nil, opt)
		th.ExpectValue(t, appliedCnt.Load(), 2)
	})
}

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
