package disttrace

import (
	tracert "github.com/aeden/traceroute"
	"github.com/google/uuid"
	"time"
)

// GenericConfig holds a master or slave configuration
type GenericConfig struct {
	*MasterConfig
	*SlaveConfig
}

// SlaveConfig holds the configuration for a dist-traceroute-slave
type SlaveConfig struct {
	MasterHost string                    `json:"-" valid:"-"`
	MasterPort string                    `json:"-" valid:"-"`
	Targets    map[uuid.UUID]TraceTarget `valid:"-"`
	Retries    int                       `valid:"int,	required,	range(0|10)"`
	MaxHops    int                       `valid:"int,	required,	range(1|100)"`
	TimeoutMs  int                       `valid:"int,	required,	range(1|10000)"`
}

// MasterConfig holds the configuration for a dist-traceroute-master
type MasterConfig struct {
	Slaves []SlaveCredentials
}

// SlaveCredentials hold authentication information for slaves on master
type SlaveCredentials struct {
	Name     string `valid:"alphanum,	required"`
	Password string `valid:"alphanum,	required"`
}

// TraceTarget contains information about a single dist-traceroute target
type TraceTarget struct {
	Name    string `valid:"alphanum,	required"`
	Address string `valid:"ipv4,		required"`
}

// TraceResult holds all relevant information of a single traceroute run
type TraceResult struct {
	Creds    SlaveCredentials        `valid:"				required"`
	ID       uuid.UUID               `valid:"				required"`
	DateTime time.Time               `valid:"				required"`
	Target   TraceTarget             `valid:"		 		required"`
	Success  bool                    `valid:"				required"`
	HopCount int                     `valid:"int,		 	required, 	range(1|100)"`
	Hops     []tracert.TracerouteHop `valid:"				required"`
}

// SubmitResult holds information about success or failure of submission of result(s)
type SubmitResult struct {
	Success       bool
	Error         string
	RetryPossible bool
}
