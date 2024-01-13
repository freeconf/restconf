package estream

import (
	"reflect"
	"time"

	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/val"
)

//go:generate go run codegen.go

type api struct{}

// Manage is implementation of ietf-subscribed-notifications.yang
func Manage(s *Service) node.Node {
	var api api
	timePtr := reflect.TypeOf(&time.Time{})
	return &nodeutil.Node{
		Object: s,
		OnRead: func(p *nodeutil.Node, m meta.Definition, t reflect.Type, v reflect.Value) (reflect.Value, error) {
			if t == timePtr {
				if v.Interface().(*time.Time).IsZero() {
					return node.NO_VALUE, nil
				}
			}
			return v, nil
		},
		OnNewNode: func(p *nodeutil.Node, m meta.Meta, o any) (node.Node, error) {
			switch x := o.(type) {
			case *Subscription:
				return api.subscription(p, m, x)
			}
			return p.DoNewNode(m, o)
		},
		OnChild: func(p *nodeutil.Node, r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "subscriptions":
				return p.New(r.Meta, s.subscriptions)
			case "filters":
				return p.New(r.Meta, s.filters)
			}
			return p.DoChild(r)
		},
		OnNotify: func(p *nodeutil.Node, r node.NotifyRequest) (node.NotifyCloser, error) {
			switch r.Meta.Ident() {
			case "subscription-suspended":
				return api.eventListener(s, SubEventSuspended, r), nil
			case "subscription-terminated":
				return api.eventListener(s, SubEventTerminated, r), nil
			case "replay-completed":
				return api.eventListener(s, SubEventCompleted, r), nil
			case "subscription-modified":
				return api.eventListener(s, SubEventModified, r), nil
			case "subscription-resumed":
				return api.eventListener(s, SubEventResumed, r), nil
			case "subscription-started":
				return api.eventListener(s, SubEventStarted, r), nil
			}
			return nil, nil
		},
	}
}

func (api api) subscription(p *nodeutil.Node, m meta.Meta, s *Subscription) (node.Node, error) {
	opts := s.Options()
	base, err := p.New(m, &opts)
	if err != nil {
		return nil, err
	}
	return &nodeutil.Extend{
		Base: base,
		OnField: func(p node.Node, r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "subscription-id", "id":
				hnd.Val = val.String(s.Id)
			case "replay-start-time-revision":
				// TODO
			default:
				return p.Field(r, hnd)
			}
			return nil
		},
	}, nil
}

func (api api) eventListener(s *Service, etype SubEventType, r node.NotifyRequest) node.NotifyCloser {
	l := s.onEvent(func(e SubEvent) {
		if etype == e.EventId {
			r.Send(api.event(e))
		}
	})
	return l.Close
}

func (api api) event(e SubEvent) node.Node {
	return &nodeutil.Node{
		Object: e.Subscription,
		OnField: func(p *nodeutil.Node, r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "replay-previous-event-time":
				// TODO
			case "reason":
				hnd.Val = val.String(e.Reason)
			default:
				return p.DoField(r, hnd)
			}
			return nil
		},
	}
}
