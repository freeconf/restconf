package estream

import (
	"errors"
	"time"

	"github.com/freeconf/yang/node"
)

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

var ErrBufferOverflow = errors.New("event buffer full")

type Receiver func(e ReceiverEvent) (RecvState, error)

func NewBufferedReceiver(size int) (chan<- ReceiverEvent, Receiver) {
	events := make(chan ReceiverEvent, size)
	return events, func(e ReceiverEvent) (RecvState, error) {
		if len(events) == size {
			return RecvStateSuspended, ErrBufferOverflow
		}
		events <- e
		return RecvStateActive, nil
	}
}

type ReceiverEvent struct {
	Name      string
	EventTime time.Time
	Event     *node.Selection
}

func (r *receiverEntry) Reset() {
	r.ExcludedEventRecords = 0
	r.SentEventRecords = 0

	// TODO: reset connection to destination
}
