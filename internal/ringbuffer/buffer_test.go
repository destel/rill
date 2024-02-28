package ringbuffer

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func makeRwHelpers(buf *Buffer[int]) (read func(t *testing.T, cnt int), write func(t *testing.T, cnt int)) {
	var ir, iw int

	write = func(t *testing.T, cnt int) {
		t.Helper()
		for k := 0; k < cnt; k++ {
			buf.Write(iw)
			iw++
		}
	}

	read = func(t *testing.T, cnt int) {
		t.Helper()

		if ir >= iw {
			_, ok := buf.Read()
			th.ExpectValue(t, ok, false)
			return
		}

		for k := 0; k < cnt; k++ {
			v, ok := buf.Read()

			if ir < iw {
				th.ExpectValue(t, ok, true)
				th.ExpectValue(t, v, ir)
				ir++
			} else {
				th.ExpectValue(t, ok, false)
			}
		}
	}

	return
}

func TestReadWrite(t *testing.T) {
	var buf Buffer[int]
	read, write := makeRwHelpers(&buf)

	th.ExpectValue(t, buf.Len(), 0)
	th.ExpectValue(t, buf.Cap(), 0)

	read(t, 5) // read from empty buffer

	th.ExpectValue(t, buf.Len(), 0)
	th.ExpectValue(t, buf.Cap(), 0)

	write(t, 100)

	th.ExpectValue(t, buf.Len(), 100)
	th.ExpectValue(t, buf.Cap(), 128)

	read(t, 50)

	th.ExpectValue(t, buf.Len(), 50)
	th.ExpectValue(t, buf.Cap(), 128)

	write(t, 50)

	th.ExpectValue(t, buf.Len(), 100)
	th.ExpectValue(t, buf.Cap(), 128)

	read(t, 100)

	th.ExpectValue(t, buf.Len(), 0)
	th.ExpectValue(t, buf.Cap(), 128)
}

func TestGrowAndShrink(t *testing.T) {
	var buf Buffer[int]
	read, write := makeRwHelpers(&buf)

	write(t, 120)
	read(t, 120)
	write(t, 20)

	if buf.offset+buf.size < len(buf.data) {
		t.Fatalf("test is not properly set up, buffer must be wrapped around")
	}

	th.ExpectValue(t, buf.Len(), 20)
	th.ExpectValue(t, buf.Cap(), 128)

	buf.Shrink()
	th.ExpectValue(t, buf.Cap(), 64)

	buf.Shrink()
	th.ExpectValue(t, buf.Cap(), 32)

	buf.Shrink()
	th.ExpectValue(t, buf.Cap(), 32)

	// empty buffer and try to shrink to min size
	read(t, 20)
	th.ExpectValue(t, buf.Len(), 0)

	buf.Shrink()
	th.ExpectValue(t, buf.Cap(), 16)

	buf.Shrink()
	th.ExpectValue(t, buf.Cap(), 16)
}

func TestCompact(t *testing.T) {
	var buf Buffer[int]
	read, write := makeRwHelpers(&buf)

	write(t, 120)
	read(t, 120)
	write(t, 20)

	th.ExpectValue(t, buf.Len(), 20)
	th.ExpectValue(t, buf.Cap(), 128)

	buf.Compact()
	th.ExpectValue(t, buf.Cap(), 32)

	// empty buffer and try to shrink to min size
	read(t, 20)
	th.ExpectValue(t, buf.Len(), 0)

	buf.Compact()
	th.ExpectValue(t, buf.Cap(), 16)
}

func TestPeekAndDiscard(t *testing.T) {
	var buf Buffer[int]

	buf.Write(10)
	buf.Write(11)

	v, ok := buf.Peek()
	th.ExpectValue(t, ok, true)
	th.ExpectValue(t, v, 10)

	buf.Discard()

	v, ok = buf.Peek()
	th.ExpectValue(t, ok, true)
	th.ExpectValue(t, v, 11)

	buf.Discard()

	_, ok = buf.Peek()
	th.ExpectValue(t, ok, false)

	buf.Discard()

	_, ok = buf.Peek()
	th.ExpectValue(t, ok, false)

}

func TestReset(t *testing.T) {
	var buf Buffer[int]

	for i := 0; i < 100; i++ {
		buf.Write(i)
	}

	buf.Reset()

	th.ExpectValue(t, buf.Len(), 0)
	th.ExpectValue(t, buf.Cap(), 128)
}
