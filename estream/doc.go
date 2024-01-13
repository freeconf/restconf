package estream

// Event Streams. Implements basic of NETCONF Notifications RFC5277 and also the backend
// for YANG Push Notifications RFC8639.
//
// Both RFCs use "named" event stream where names are assigned on
// the server side (publisher) and correlate to YANG notification paths OR reserved
// named streams like "NETCONF".
//
