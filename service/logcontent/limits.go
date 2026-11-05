package logcontent

const (
	maxStoredBody = 1 << 20
	maxParsedBody = 8 << 20
	maxAssembled  = 1 << 20

	// tapMarginBytes is how far past maxStoredBody a head-only copy may run, so
	// a key straddling the cut is still whole when the worker redacts it.
	tapMarginBytes = 64 << 10

	sseHeadBytes = 512 << 10
	sseTailBytes = 512 << 10

	foldChunkBytes  = 32 << 10
	foldQueueChunks = 16

	// reusedFrameBytes caps the frame buffer an assembler keeps between frames.
	reusedFrameBytes = 64 << 10
)
