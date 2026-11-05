package common

// LogContentCaptureEnabled switches per-call content capture on or off.
var LogContentCaptureEnabled = true

// LogContentRetentionDays is how long captured call content is kept.
var LogContentRetentionDays = 30

const (
	LogContentRetentionDaysMin = 1
	LogContentRetentionDaysMax = 3650
)
