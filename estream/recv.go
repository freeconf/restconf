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

type Receiver func(e ReceiverEvent) error

type receiverSubscription interface {
	activateReceiver(r *receiverEntry, active bool, reason string)
}

var ErrBufferOverflow = errors.New("event buffer full")

type ReceiverEvent struct {
	Name      string
	EventTime time.Time
	Event     *node.Selection
}

type receiverEntry struct {
	Name                 string
	sub                  receiverSubscription
	State                RecvState
	ExcludedEventRecords int64
	SentEventRecords     int64
	receiver             Receiver
}

func (r *receiverEntry) Reset() {
	// TODO: not sure spec says to do this
	r.ExcludedEventRecords = 0
	r.SentEventRecords = 0

	r.sub.activateReceiver(r, true, "")
}
