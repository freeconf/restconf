package gateway

import (
	"strings"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/val"

	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

func RegistrarNode(registrar Registrar) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "registrations":
				return registrationsNode(registrar), nil
			}
			return nil, nil
		},
		OnAction: func(r node.ActionRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "register":
				var reg Registration
				if err := r.Input.InsertInto(regNode(&reg)).LastErr; err != nil {
					return nil, err
				}
				ctx := r.Selection.Context
				if regAddr, hasRegAddr := ctx.Value(device.RemoteIpAddressKey).(string); hasRegAddr {
					reg.Address = strings.Replace(reg.Address, "{REQUEST_ADDRESS}", regAddr, 1)
				}
				registrar.RegisterDevice(reg.DeviceId, reg.Address)
				return nil, nil
			}
			return nil, nil
		},
		OnNotify: func(r node.NotifyRequest) (node.NotifyCloser, error) {
			switch r.Meta.Ident() {
			case "update":
				sub := registrar.OnRegister(func(reg Registration) {
					r.Send(regNode(&reg))
				})
				return sub.Close, nil
			}
			return nil, nil
		},
	}
}

func registrationsNode(registrar Registrar) node.Node {

	// assume local registrar, need better way to iterate
	index := node.NewIndex(registrar.(*LocalRegistrar).regs)

	return &nodeutil.Basic{
		OnNextItem: func(r node.ListRequest) nodeutil.BasicNextItem {
			var reg Registration
			var found bool
			return nodeutil.BasicNextItem{
				GetByKey: func() error {
					reg, found = registrar.LookupRegistration(r.Key[0].String())
					return nil
				},
				GetByRow: func() ([]val.Value, error) {
					if r.Row < registrar.RegistrationCount() {
						if v := index.NextKey(r.Row); v != node.NO_VALUE {
							id := v.String()
							reg, found = registrar.LookupRegistration(id)
							key := []val.Value{val.String(reg.DeviceId)}
							return key, nil
						}
					}
					return nil, nil
				},
				Node: func() (node.Node, error) {
					if found {
						return nodeutil.ReflectChild(&reg), nil
					}
					return nil, nil
				},
			}
		},
	}
}

func regNode(reg *Registration) node.Node {
	return nodeutil.ReflectChild(reg)
}
