package buffers

import (
	"sync"
)

// ============================================================================
// BUFFER POOL - Reusable buffers for registry operations
// ============================================================================

var (
	// Pool for UTF-16 name buffers for subkey names (256 uint16)
	nameBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]uint16, 256)
			return &buf
		},
	}

	// Pool for UTF-16 name buffers for value names (16384 uint16)
	largeNameBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]uint16, 16384)
			return &buf
		},
	}

	// Pool for data buffers (16KB for values)
	dataBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 16384)
			return &buf
		},
	}

	// Pool for large data buffers (64KB for large values)
	largeDataBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 65536)
			return &buf
		},
	}
)

func GetNameBuffer() *[]uint16 {
	return nameBufferPool.Get().(*[]uint16)
}

func PutNameBuffer(buf *[]uint16) {
	for i := range *buf {
		(*buf)[i] = 0
	}
	nameBufferPool.Put(buf)
}

func GetDataBuffer() *[]byte {
	return dataBufferPool.Get().(*[]byte)
}

func PutDataBuffer(buf *[]byte) {
	for i := range *buf {
		(*buf)[i] = 0
	}
	dataBufferPool.Put(buf)
}

func GetLargeNameBuffer() *[]uint16 {
	return largeNameBufferPool.Get().(*[]uint16)
}

func PutLargeNameBuffer(buf *[]uint16) {
	for i := range *buf {
		(*buf)[i] = 0
	}
	largeNameBufferPool.Put(buf)
}

func GetLargeDataBuffer() *[]byte {
	return largeDataBufferPool.Get().(*[]byte)
}

func PutLargeDataBuffer(buf *[]byte) {
	for i := range *buf {
		(*buf)[i] = 0
	}
	largeDataBufferPool.Put(buf)
}
