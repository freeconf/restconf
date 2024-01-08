package estream

import "github.com/freeconf/yang/node"

type RecvState int

const (
	RecvStateDisconnected = iota
	RecvStateActive
	RecvStateSuspended
	RecvStateConnecting
)

type receiverEntry struct {
	Name                 string
	State                RecvState
	ExcludedEventRecords int64
	SentEventRecords     int64
	receiver             Receiver
}

type Receiver func(e ReceiverEvent) RecvState

func NewBufferedReceiver(size int) (chan<- ReceiverEvent, Receiver) {
	events := make(chan ReceiverEvent, size)
	return events, func(e ReceiverEvent) RecvState {
		if len(events) == size {
			return RecvStateSuspended
		}
		events <- e
		return RecvStateActive
	}
}

type ReceiverEvent struct {
	Name  string
	Event *node.Selection
}

func (r *receiverEntry) Reset() {
	r.ExcludedEventRecords = 0
	r.SentEventRecords = 0

	// TODO: reset connection to destination
}
