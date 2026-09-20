package rill

import (
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

		in := FromSlice([]int{1, 2, 3}, nil)
		Discard(in, opt, nil, opt)
		th.ExpectValue(t, appliedCnt.Load(), 2)
	})
}
