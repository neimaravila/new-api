package logcontent

import (
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const defaultCaptureMemoryMB = 256

// The global memory budget for copies in flight: every copy reserves before
// it grows and keeps the reservation until its record is compressed. Without
// budget the copy stops; the call never waits.
var (
	budgetLimit     atomic.Int64
	budgetLimitOnce sync.Once
	budgetInUse     atomic.Int64
)

// loadBudgetLimit reads LOG_CONTENT_CAPTURE_MEMORY_MB on first use, after the
// environment is loaded; values under 1 mean the default.
func loadBudgetLimit() int64 {
	budgetLimitOnce.Do(func() {
		mb := common.GetEnvOrDefault("LOG_CONTENT_CAPTURE_MEMORY_MB", defaultCaptureMemoryMB)
		if mb < 1 {
			mb = defaultCaptureMemoryMB
		}
		budgetLimit.Store(int64(mb) << 20)
	})
	return budgetLimit.Load()
}

// tryReserve takes n bytes of the budget, or nothing when they do not fit.
func tryReserve(n int64) bool {
	if n <= 0 {
		return true
	}
	limit := loadBudgetLimit()
	for {
		inUse := budgetInUse.Load()
		if inUse+n > limit {
			return false
		}
		if budgetInUse.CompareAndSwap(inUse, inUse+n) {
			return true
		}
	}
}

// release returns n bytes taken by tryReserve.
func release(n int64) {
	if n > 0 {
		budgetInUse.Add(-n)
	}
}
