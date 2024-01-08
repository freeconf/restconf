package estream

// Managing subscriptions to "named" event stream.  Names are assigned on
// the server side (publisher) and correlate to YANG notification paths OR reserved
// named streams like "NETCONF".
//
// Per the RFC8639 2.1 Event Streams:
//
// Identifying:
///   a) how event streams are defined (other than the NETCONF stream)
//    b) how event records are defined/generated
//    c) how event records are assigned to event streams is out of scope for this document.
//
