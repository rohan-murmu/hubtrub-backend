package util

import (
	"time"
)

const (
	// Message constraints
	MaxMessageSize = 2048
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second    // Time to wait for a pong response
	PingPeriod     = (PongWait * 9) / 10 // Frequency of ping messages
	SendBufferSize = 256
)
