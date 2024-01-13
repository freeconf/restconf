package estream

import (
	"container/list"
	"fmt"
	"strconv"
	"time"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/nodeutil"
)

type Service struct {
	subscriptions      map[string]*Subscription
	filters            map[string]Filter
	streams            map[string]Stream
	listeners          *list.List
	subcriptionCounter int64
}

func NewService() *Service {
	return &Service{
		subscriptions:      make(map[string]*Subscription),
		filters:            make(map[string]Filter),
		streams:            make(map[string]Stream),
		listeners:          list.New(),
		subcriptionCounter: 100, // starting at zero or one seems disconcerting
	}
}

func (s *Service) AddFilter(f Filter) {
	s.filters[f.Name] = f
}

func (s *Service) AddStream(stream Stream) {
	s.streams[stream.Name] = stream
}

type eventListener func(e SubEvent)

func (s *Service) onEvent(l eventListener) nodeutil.Subscription {
	return nodeutil.NewSubscription(s.listeners, s.listeners.PushBack(l))
}

type EstablishRequest struct {
	Stream           string
	StreamFilterName string
	ReplayStartTime  time.Time
	StopTime         time.Time
}

func (s *Service) EstablishSubscription(req EstablishRequest) (*Subscription, error) {
	sub := &Subscription{
		Id: s.nextSubId(),
	}
	var opts SubscriptionOptions
	if err := s.updateFilter(&opts, req.StreamFilterName); err != nil {
		return nil, err
	}
	if err := s.updateStream(&opts, req.Stream); err != nil {
		return nil, err
	}
	if err := sub.Apply(opts); err != nil {
		return nil, err
	}
	s.subscriptions[sub.Id] = sub
	s.updateListeners(SubEvent{Subscription: sub, EventId: SubEventStarted})
	return sub, nil
}

func (s *Service) updateFilter(opts *SubscriptionOptions, filterName string) error {
	if filterName == "" {
		opts.Filter = Filter{}
	} else {
		f, found := s.filters[filterName]
		if !found {
			return fmt.Errorf("filter %w %s", fc.NotFoundError, filterName)
		}
		opts.Filter = f
	}
	return nil
}

func (s *Service) updateStream(opts *SubscriptionOptions, streamName string) error {
	if streamName == "" {
		opts.Stream = Stream{}
	} else {
		s, found := s.streams[streamName]
		if !found {
			return fmt.Errorf("stream %w %s", fc.NotFoundError, streamName)
		}
		opts.Stream = s
	}
	return nil
}

type ModifyRequest struct {
	SubscriptionId   string
	StreamFilterName string
	StopTime         time.Time
}

func (s *Service) ModifySubscription(req ModifyRequest) error {
	sub, found := s.subscriptions[req.SubscriptionId]
	if !found {
		return fmt.Errorf("subscription %w %s", fc.NotFoundError, req.SubscriptionId)
	}
	opts := sub.Options()
	opts.StopTime = req.StopTime
	if err := s.updateFilter(&opts, req.StreamFilterName); err != nil {
		return err
	}
	if err := sub.Apply(opts); err != nil {
		return err
	}
	s.updateListeners(SubEvent{Subscription: sub, EventId: SubEventModified})
	return nil
}

func (s *Service) updateListeners(e SubEvent) {
	for l := s.listeners.Front(); l != nil; l = l.Next() {
		l.Value.(eventListener)(e)
	}
}

func (s *Service) KillSubscription(subId string) error {
	return nil
}

func (s *Service) DeleteSubsccription(subId string) error {
	return nil
}

func (s *Service) nextSubId() string {
	var id int64
	s.subcriptionCounter, id = s.subcriptionCounter+1, s.subcriptionCounter
	return strconv.FormatInt(id, 10)
}
