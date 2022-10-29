package device

import (
	"github.com/freeconf/yang/val"

	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

type ProxyContextKey int

const RemoteIpAddressKey ProxyContextKey = 0

type LocalLocationService map[string]string

func MapNode(mgr Map) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "device":
				return deviceRecordListNode(mgr), nil
			}
			return nil, nil
		},
		OnNotify: func(r node.NotifyRequest) (node.NotifyCloser, error) {
			switch r.Meta.Ident() {
			case "update":
				sub := mgr.OnUpdate(func(d Device, id string, c Change) {
					n := deviceChangeNode(id, d, c)
					r.Send(n)
				})
				return sub.Close, nil
			}
			return nil, nil
		},
	}
}

func deviceChangeNode(id string, d Device, c Change) node.Node {
	return &nodeutil.Extend{
		Base: deviceNode(id, d),
		OnField: func(p node.Node, r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "change":
				var err error
				hnd.Val, err = node.NewValue(r.Meta.Type(), int(c))
				if err != nil {
					return err
				}
			default:
				return p.Field(r, hnd)
			}
			return nil
		},
	}
}

func deviceRecordListNode(devices Map) node.Node {
	return &nodeutil.Basic{
		OnNextItem: func(r node.ListRequest) nodeutil.BasicNextItem {
			var d Device
			var id string
			var err error
			return nodeutil.BasicNextItem{
				GetByKey: func() error {
					id = r.Key[0].String()
					d, err = devices.Device(id)
					return err
				},
				GetByRow: func() ([]val.Value, error) {
					if r.Row >= devices.Len() {
						return nil, nil
					}
					id = devices.NthDeviceId(r.Row)
					d, err = devices.Device(id)
					return []val.Value{val.String(id)}, err
				},
				Node: func() (node.Node, error) {
					if d != nil {
						return deviceNode(id, d), nil
					}
					return nil, nil
				},
			}
		},
	}
}

func deviceNode(id string, d Device) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "module":
				return deviceModuleList(d.Modules()), nil
			}
			return nil, nil
		},
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "deviceId":
				hnd.Val = val.String(id)
			}
			return nil
		},
	}
}

func deviceModuleList(mods map[string]*meta.Module) node.Node {
	index := node.NewIndex(mods)
	return &nodeutil.Basic{
		OnNextItem: func(r node.ListRequest) nodeutil.BasicNextItem {
			var m *meta.Module
			return nodeutil.BasicNextItem{
				GetByKey: func() error {
					m = mods[r.Key[0].String()]
					return nil
				},
				GetByRow: func() ([]val.Value, error) {
					if v := index.NextKey(r.Row); v != node.NO_VALUE {
						module := v.String()
						if m = mods[module]; m != nil {
							return []val.Value{val.String(m.Ident())}, nil
						}
					}
					return nil, nil
				},
				Node: func() (node.Node, error) {
					if m != nil {
						return deviceModuleNode(m), nil
					}
					return nil, nil
				},
			}
		},
	}
}

func deviceModuleNode(m *meta.Module) node.Node {
	return &nodeutil.Basic{
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "name":
				hnd.Val = val.String(m.Ident())
			case "revision":
				hnd.Val = val.String(m.Revision().Ident())
			}
			return nil
		},
	}
}
