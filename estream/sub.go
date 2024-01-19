package estream

import (
	"errors"
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
	Subscription *Subscription
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

type subService interface {
	updateListeners(e SubEvent)
}

type Subscription struct {
	Id      string
	closer  node.NotifyCloser
	opts    SubscriptionOptions
	service subService

	ConfiguredSubscriptionState SubState
	Recievers                   map[string]*receiverEntry
}

func NewSubscription(id string, service subService) *Subscription {
	return &Subscription{
		Id:        id,
		service:   service,
		Recievers: make(map[string]*receiverEntry),
	}
}

func (s *Subscription) AddReceiver(name string, receiver Receiver) error {
	if _, exists := s.Recievers[name]; exists {
		return errors.New("receiver already exists")
	}
	s.Recievers[name] = &receiverEntry{
		sub:      s,
		Name:     name,
		receiver: receiver,
		State:    RecvStateActive,
	}
	return nil
}

func (s *Subscription) RemoveReceiver(name string) error {
	delete(s.Recievers, name)
	return nil
}

func (s *Subscription) Options() SubscriptionOptions {
	return s.opts
}

func (s *Subscription) activateReceiver(r *receiverEntry, active bool, reason string) {
	e := SubEvent{
		EventId:      SubEventStarted,
		Subscription: s,
	}
	if active {
		r.State = RecvStateActive
		e.EventId = SubEventResumed
	} else {
		r.State = RecvStateSuspended
		e.EventId = SubEventSuspended
		e.Reason = reason
	}
	s.service.updateListeners(e)
}

func (s *Subscription) Apply(opts SubscriptionOptions) error {
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
				err = r.receiver(ReceiverEvent{
					Name:      r.Name,
					EventTime: n.EventTime,
					Event:     eventSel,
				})
				if err == nil {
					r.SentEventRecords++
					continue
				} else {
					s.activateReceiver(r, false, err.Error())
				}
			}
			r.ExcludedEventRecords++

		}
	})
	return err
}
