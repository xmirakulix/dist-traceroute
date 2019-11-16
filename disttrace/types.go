package disttrace

import (
	"errors"
	"github.com/google/uuid"
	"time"
)

import (
	tracert "github.com/aeden/traceroute"
	valid "github.com/asaskevich/govalidator"
)

// GenericConfig holds a master or slave configuration
type GenericConfig struct {
	*MasterConfig
	*SlaveConfig
}

// SlaveConfig holds the configuration for a dist-traceroute-slave
type SlaveConfig struct {
	MasterHost string        `json:"-" valid:"-"`
	MasterPort string        `json:"-" valid:"-"`
	Targets    []TraceTarget `valid:"-"`
}

// MasterConfig holds the configuration for a dist-traceroute-master
type MasterConfig struct {
	Slaves []SlaveCredentials
}

// SlaveCredentials hold authentication information for slaves on master
type SlaveCredentials struct {
	ID       uuid.UUID `valid:"-"`
	Name     string    `valid:"alphanum,	required"`
	Password string    `valid:"alphanum,	required"`
}

// TraceTarget contains information about a single dist-traceroute target
type TraceTarget struct {
	ID        uuid.UUID `valid:"-"`
	Name      string    `valid:"alphanum,	required"`
	Address   string    `valid:"host,		required"`
	Retries   int       `valid:"int,	required,	range(0|10)"`
	MaxHops   int       `valid:"int,	required,	range(1|100)"`
	TimeoutMs int       `valid:"int,	required,	range(1|10000)"`
}

// TraceResult holds all relevant information of a single traceroute run
type TraceResult struct {
	Creds    SlaveCredentials        `valid:"		required"`
	ID       uuid.UUID               `valid:"-"`
	DateTime time.Time               `valid:"-"`
	Target   TraceTarget             `valid:"		required"`
	Success  bool                    `valid:"-"`
	HopCount int                     `valid:"int,	required, 	range(1|100)"`
	Hops     []tracert.TracerouteHop `valid:"-"`
}

// SubmitResult holds information about success or failure of submission of result(s)
type SubmitResult struct {
	Success       bool
	Error         string
	RetryPossible bool
}

// ValidateTraceResult validates contents of a TraceResult
func ValidateTraceResult(res TraceResult) (bool, error) {

	// check credentials and tracetargets
	if ok, err := valid.ValidateStruct(res); !ok || err != nil {
		return false, err
	}

	for _, hop := range res.Hops {
		// check if IP is valid
		if !valid.IsIP(hop.AddressString()) {
			return false, errors.New("Invalid IP Address: " + hop.AddressString())
		}

		// check if hostname is valid if present
		if hop.Host != "" && !valid.IsDNSName(hop.Host) {
			return false, errors.New("Invalid IP Address: " + hop.AddressString())
		}
	}

	log.Debug("ValidateTraceResult: Results are valid")
	return true, nil
}
