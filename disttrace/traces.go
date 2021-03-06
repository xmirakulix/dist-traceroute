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

// TraceResult holds all relevant information of a single traceroute run
type TraceResult struct {
	Slave    Slave                   `valid:"		required"`
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

	log.Debugf("ValidateTraceResult: Validating trace results for target '%v' from slave '%v'", res.Target, res.Slave.Name)
	// check credentials and tracetargets
	if ok, err := valid.ValidateStruct(res); !ok || err != nil {
		return false, err
	}

	for _, hop := range res.Hops {
		// check if IP is valid
		if !valid.IsIP(hop.AddressString()) {
			log.Debug("ValidateTraceResult: Invalid IP Address: ", hop.AddressString())
			return false, errors.New("Invalid IP Address: " + hop.AddressString())
		}

		// check if hostname is valid if present
		if hop.Host != "" && !valid.IsDNSName(hop.Host) {
			log.Debug("ValidateTraceResult: Invalid IP Address: ", hop.AddressString())
			return false, errors.New("Invalid IP Address: " + hop.AddressString())
		}
	}

	log.Debug("ValidateTraceResult: Results are valid")
	return true, nil
}
