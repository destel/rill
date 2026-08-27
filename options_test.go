package rill

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestCollectSinkOptions(t *testing.T) {
	t.Run("nil options are ignored", func(t *testing.T) {
		_, opt := Settlement()

		opts := collectSinkOptions([]SinkOption{nil, opt, nil})
		th.ExpectValue(t, len(opts.settleChans), 1)
	})
}
