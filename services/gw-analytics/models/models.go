package models

import "time"

type AnalyticsEvent struct {
	TransactionID string
	UserID        string
	Operation     string
	Status        string
	CreatedAt     time.Time
	ReceivedAt    time.Time
	LatencyMs     uint64
	RetryCount    uint32
	Error         string
}
