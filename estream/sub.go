package estream

import (
	"time"

	"github.com/freeconf/yang/node"
)

type SubEventType int

const (
	SubEventSuspended SubEventType = iota
	SubEventTerminated
	SubEventCompleted
	SubEventModified
	SubEventResumed
	SubEventStarted
)

type SubEvent struct {
	EventId      SubEventType
	Subscription *subscription
	Reason       string
}

type SubState int

const (
	SubStateValid = iota
	SubStateInvalid
	SubStateConcluded
)

type SubscriptionOptions struct {
	Filter          Filter
	Stream          Stream
	ReplayStartTime time.Time
	// configured-replay
	ReplayStartTimeRevision time.Time
	StopTime                time.Time
	// transport
	// encoding
	Purpose       string
	SourceAddress string
}

type subscription struct {
	Id     string
	closer node.NotifyCloser
	opts   SubscriptionOptions

	ConfiguredSubscriptionState SubState
	Recievers                   map[string]*receiverEntry
}

func (s *subscription) Options() SubscriptionOptions {
	return s.opts
}

func (s *subscription) Apply(opts SubscriptionOptions) error {
	if s.closer != nil {
		s.closer()
	}
	notifySel, err := opts.Stream.Open()
	if err != nil {
		return err
	}
	s.opts = opts
	s.closer, err = notifySel.Notifications(func(n node.Notification) {
		eventSel := n.Event
		// TODO: Compare event time to stop time and filter accordingly
		if !s.opts.Filter.Empty() {
			eventSel = s.opts.Filter.Filter(eventSel)
		}
		for _, r := range s.Recievers {
			if eventSel != nil && r.State == RecvStateActive {
				r.State = r.receiver(ReceiverEvent{
					Name:  r.Name,
					Event: eventSel,
				})
				r.SentEventRecords++
			} else {
				r.ExcludedEventRecords++
			}
		}
	})
	return err
}
