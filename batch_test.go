package rill

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	th.TestVariants(t, "timeout", []time.Duration{5 * time.Second, -1}, func(t *testing.T, timeout time.Duration) {
		t.Run("nil", func(t *testing.T) {
			th.ExpectValue(t, Batch[string](nil, 10, timeout), nil)
		})

		t.Run("empty", func(t *testing.T) {
			in := make(chan Try[int])
			close(in)

			out := Batch(in, 10, timeout)
			outSlice := toUnifiedStringSlice(out, "%v")
			th.ExpectSlice(t, outSlice, nil)
		})
	})

	t.Run("zero timeout panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic")
			}
		}()
		Batch[string](nil, 10, 0)
	})

	th.RunSynctest(t, "with timeout", func(t *testing.T) {
		in := Generate(func(send func(int), sendError func(error)) {
			// flush by size: the mid-batch pause is too short to trigger the timeout
			send(11)
			send(12)
			send(13)
			time.Sleep(4 * time.Second)
			send(14)

			// flush by timeout: the deadline is 5s after the first item - it hits during the second sleep
			send(21)
			time.Sleep(4 * time.Second)
			send(22)
			send(23)
			time.Sleep(4 * time.Second)

			// no batch is open - the idle gap flushes nothing
			time.Sleep(10 * time.Second)

			// flush by error: the partial batch is flushed first, then the error is forwarded
			send(31)
			time.Sleep(4 * time.Second)
			send(32)
			sendError(fmt.Errorf("err1"))

			// flush by timeout again: the error disarmed the previous deadline,
			// otherwise [41] would have flushed alone before 42 arrived
			send(41)
			time.Sleep(4 * time.Second)
			send(42)
			time.Sleep(4 * time.Second)

			// errors hitting an empty batch are forwarded alone
			sendError(fmt.Errorf("err2"))
			sendError(fmt.Errorf("err3"))

			// trailing partial is flushed when the input closes
			send(51)
			send(52)
		})

		out := Batch(in, 4, 5*time.Second)

		outSlice := toUnifiedStringSlice(out, "%v")
		th.ExpectSlice(t, outSlice, []string{
			"[11 12 13 14]", "[21 22 23]", "[31 32]", "err1", "[41 42]", "err2", "err3", "[51 52]",
		})
	})

	th.RunSynctest(t, "no timeout", func(t *testing.T) {
		in := Generate(func(send func(int), sendError func(error)) {
			// flush by size: sleep has no effect
			send(11)
			send(12)
			send(13)
			time.Sleep(30 * time.Second)
			send(14)

			// flush by error: the partial batch is flushed first, then the error is forwarded
			send(21)
			send(22)
			sendError(fmt.Errorf("err1"))

			// errors hitting an empty batch are forwarded alone
			sendError(fmt.Errorf("err2"))
			sendError(fmt.Errorf("err3"))

			// trailing partial is flushed when the input closes
			send(31)
			send(32)
		})

		out := Batch(in, 4, -1)

		outSlice := toUnifiedStringSlice(out, "%v")
		th.ExpectSlice(t, outSlice, []string{
			"[11 12 13 14]", "[21 22]", "err1", "err2", "err3", "[31 32]",
		})
	})
}

func TestUnbatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Unbatch[string](nil), nil)
	})

	th.RunSynctest(t, "correctness", func(t *testing.T) {
		in := Generate(func(send func([]int), sendError func(error)) {
			send([]int{1, 2})
			sendError(fmt.Errorf("err1"))
			send([]int{10, 11, 12})
			send([]int{13, 14})
			sendError(fmt.Errorf("err2"))
			send([]int{20, 21})
		})

		out := Unbatch(in)

		outSlice := toUnifiedStringSlice(out, "%v")
		th.ExpectSlice(t, outSlice, []string{
			"1", "2", "err1", "10", "11", "12", "13", "14", "err2", "20", "21",
		})
	})

	th.RunSynctest(t, "inverse of Batch", func(t *testing.T) {
		in := Generate(func(send func(int), sendError func(error)) {
			send(0)
			send(1)
			send(2)
			send(3)
			send(4)
			sendError(fmt.Errorf("err5"))
			send(6)
			sendError(fmt.Errorf("err7"))
			send(8)
			send(9)
		})

		out := Unbatch(Batch(in, 3, -1))

		outSlice := toUnifiedStringSlice(out, "%v")
		th.ExpectSlice(t, outSlice, []string{"0", "1", "2", "3", "4", "err5", "6", "err7", "8", "9"})
	})
}
