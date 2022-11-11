package restconf

// Compliance is the global variable that sets the default behavior if the
// FreeCONF RESTCONF library.
//
// By default this is for strict IETF compliance!
//
// This sets just the default behavior of data structures, each individual
// instance should allow for controlling the compliance of that instance should
// you need to have instances in different modes at the same time.
//
// If you wish change default compliance, be sure to do it at the beginning of your
// to application before any objects are constructed.
var Compliance = ComplianceOptions{}

// LegacyCompliance are the settings pre 2023 before true IETF compliance was
// attempted. To use this:
//
//  restconf.Compliance = restconf.LegacyCompliance
//
// or you can just set individual settings on restconf.Compliance global variable
var LegacyCompliance = ComplianceOptions{
	ServeOperationsUnderData: true,
}

// ComplianceOptions hold all the compliance settings
type ComplianceOptions struct {

	// allow rpc to serve under /restconf/data/{module:}/{rpc} which while intuative and
	// original design, it is not in compliance w/RESTCONF spec
	ServeOperationsUnderData bool
}
