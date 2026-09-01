package rill

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestSinkOption(t *testing.T) {
	t.Run("nil options are ignored", func(t *testing.T) {
		_, opt := Settlement()

		opts := collectSinkOptions([]SinkOption{nil, opt, nil})
		th.ExpectValue(t, len(opts.settleFuncs), 1)
	})

	t.Run("settlement can't be reused", func(t *testing.T) {
		_, opt := Settlement()
		_, _, _ = First(FromSlice([]int{}, nil), opt)

		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()

		_, _, _ = First(FromSlice([]int{}, nil), opt)
	})
}
