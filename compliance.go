package restconf

import "fmt"

// Compliance is the global variable that sets the default behavior if the
// FreeCONF RESTCONF library.
//
// By default this is for strict IETF compliance!
//
// This sets just the default behavior of data structures, each individual
// instance should allow for controlling the compliance of that instance should
// you need to have instances in different modes at the same time.
var Strict = ComplianceOptions{}

// Simplified are the settings pre 2023 before true IETF compliance was
// attempted. To use this:
//
//	restconf.Compliance = restconf.Simplified
//
// or you can just set individual settings on restconf.Compliance global variable.
var Simplified = ComplianceOptions{
	AllowRpcUnderData:          true,
	DisableNotificationWrapper: true,
	DisableActionWrapper:       true,
	SimpleErrorResponse:        true,
}

// ComplianceOptions hold all the compliance settings.  If you enable any of these
// settings, then you run the risk of not being complatible with other RESTCONF
// implementations
type ComplianceOptions struct {

	// allow rpc to serve under /restconf/data/{module:}/{rpc} which while intuative
	// it is not in compliance w/RESTCONF spec
	AllowRpcUnderData bool

	// IETF notification messages with extra data including
	// event time and ietf-restconf:notfication container
	// https://datatracker.ietf.org/doc/html/rfc8040#section-6.4
	DisableNotificationWrapper bool

	// IETF rpc/action inputs and outputs are wrapped with extra container
	// https://datatracker.ietf.org/doc/html/rfc8040#section-6.
	DisableActionWrapper bool

	// Errors have a specific structure
	// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.3
	SimpleErrorResponse bool
}

func (compliance ComplianceOptions) String() string {
	if compliance == Simplified {
		return "simplified"
	}
	if compliance == Strict {
		return "strict"
	}
	return fmt.Sprintf("mixed %#v", compliance)
}
