package rill

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestSinkOption(t *testing.T) {
	t.Run("nil options are ignored", func(t *testing.T) {
		scope, _ := NewScope(t.Context())
		defer scope.Cancel()

		opts := collectSinkOptions([]SinkOption{nil, scope, nil})
		th.ExpectValue(t, len(opts.settleFuncs), 1)
	})
}
