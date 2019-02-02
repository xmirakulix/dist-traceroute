package disttrace

import (
	tracert "github.com/aeden/traceroute"
	"github.com/google/uuid"
	"time"
)

// SlaveConfig holds the configuration for a dist-traceroute-slave
type SlaveConfig struct {
	ReportURL string
	Targets   map[uuid.UUID]TraceTarget
	Retries   int
	MaxHops   int
	TimeoutMs int
}

// TraceTarget contains information about a single dist-traceroute target
type TraceTarget struct {
	Name    string
	Address string
}

// TraceResult holds all relevant information of a single traceroute run
type TraceResult struct {
	ID       uuid.UUID
	DateTime time.Time
	Target   TraceTarget
	Success  bool
	HopCount int
	Hops     []tracert.TracerouteHop
}

// SubmitResult holds information about success or failure of submission of result(s)
type SubmitResult struct {
	Success       bool
	Error         string
	RetryPossible bool
}
