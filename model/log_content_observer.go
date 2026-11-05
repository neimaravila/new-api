package model

import "github.com/gin-gonic/gin"

const logRowObserverKey = "log_row_observer"

// LogRowObserver is told about each consume or error log row written for a
// call, so content capture follows the same rules as the log itself.
type LogRowObserver interface {
	OnLogRow(log *Log)
}

// SetLogRowObserver registers the observer of the call's log rows.
func SetLogRowObserver(c *gin.Context, observer LogRowObserver) {
	if c == nil || observer == nil {
		return
	}
	c.Set(logRowObserverKey, observer)
}

// notifyLogRowWritten tells the call's observer about a row that was written;
// a failed write is not reported. The observer can never break the log path.
func notifyLogRowWritten(c *gin.Context, log *Log, err error) {
	if c == nil || log == nil || err != nil {
		return
	}
	value, ok := c.Get(logRowObserverKey)
	if !ok {
		return
	}
	observer, ok := value.(LogRowObserver)
	if !ok || observer == nil {
		return
	}
	defer func() { _ = recover() }()
	observer.OnLogRow(log)
}
