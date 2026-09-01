package rill

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestSinkOption(t *testing.T) {
	t.Run("nil options are ignored", func(t *testing.T) {
		_, scope := WithContext(t.Context())
		defer scope.Cancel()

		opts := collectSinkOptions([]SinkOption{nil, scope, nil})
		th.ExpectValue(t, len(opts.settleFuncs), 1)
	})
}
