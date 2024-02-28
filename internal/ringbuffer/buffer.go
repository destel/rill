package ringbuffer

const minCap = 16

type Buffer[T any] struct {
	data         []T
	offset, size int
}

func (b *Buffer[T]) Cap() int {
	return len(b.data)
}

func (b *Buffer[T]) Len() int {
	return b.size
}

// write to end
func (b *Buffer[T]) Write(v T) {
	b.Grow(1)

	pos := (b.offset + b.size) % len(b.data)
	b.data[pos] = v
	b.size++
}

// read from start
func (b *Buffer[T]) Read() (T, bool) {
	if b.size == 0 {
		var zero T
		return zero, false
	}

	v := b.data[b.offset]
	b.Discard()
	return v, true
}

func (b *Buffer[T]) Peek() (T, bool) {
	if b.size == 0 {
		var zero T
		return zero, false
	}

	return b.data[b.offset], true
}

func (b *Buffer[T]) Discard() bool {
	if b.size == 0 {
		return false
	}

	var zero T
	b.data[b.offset] = zero // let GC do its work

	b.offset = (b.offset + 1) % len(b.data)
	b.size--
	return true
}

// change the capacity and defragment the buffer
// panics if newCap is less than buf.size
func (b *Buffer[T]) setCap(newCap int) {
	newData := make([]T, newCap)

	end := b.offset + b.size
	if end <= len(b.data) {
		copy(newData, b.data[b.offset:end])
	} else {
		copied := copy(newData, b.data[b.offset:])
		copy(newData[copied:], b.data[:b.size-copied])
	}

	b.data = newData
	b.offset = 0
}

func (b *Buffer[T]) Grow(n int) {
	targetSize := b.size + n
	targetCap := cap(b.data)

	if targetCap >= targetSize {
		return // enough
	}

	if targetCap < minCap {
		targetCap = minCap
	}
	for targetCap < targetSize {
		targetCap <<= 1 // double the capacity
	}

	b.setCap(targetCap)
}

func (b *Buffer[T]) CanShrink() bool {
	half := cap(b.data) >> 1
	return half >= minCap && half >= b.size
}

func (b *Buffer[T]) Shrink() {
	if b.CanShrink() {
		b.setCap(cap(b.data) >> 1)
	}
}

func (b *Buffer[T]) Compact() {
	targetCap := cap(b.data)
	for {
		half := targetCap >> 1
		if half >= minCap && half >= b.size {
			targetCap = half
		} else {
			break
		}
	}

	if targetCap < cap(b.data) {
		b.setCap(targetCap)
	}
}

func (b *Buffer[T]) Reset() {
	var zero T
	for i := 0; i < b.size; i++ {
		b.data[(b.offset+i)%len(b.data)] = zero
	}
	b.offset = 0
	b.size = 0
}
