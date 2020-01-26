package restconf

import (
	"github.com/freeconf/restconf/stock"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
	"github.com/freeconf/yang/val"
)

func Node(mgmt *Server, ypath source.Opener) node.Node {
	return &nodeutil.Extend{
		Base: nodeutil.ReflectChild(mgmt),
		OnChild: func(p node.Node, r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "web":
				if r.New {
					mgmt.Web = stock.NewHttpServer(mgmt)
				}
				if mgmt.Web != nil {
					return stock.WebServerNode(mgmt.Web), nil
				}
			case "callHome":
				if r.New {
					rc := ProtocolHandler(ypath)
					mgmt.CallHome = NewCallHome(rc)
				}
				if mgmt.CallHome != nil {
					return CallHomeNode(mgmt.CallHome), nil
				}
			default:
				return p.Child(r)
			}
			return nil, nil
		},
		OnField: func(p node.Node, r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "debug":
				if r.Write {
					fc.DebugLog(hnd.Val.Value().(bool))
				} else {
					hnd.Val = val.Bool(fc.DebugLogEnabled())
				}
			case "streamCount":
				hnd.Val = val.Int32(mgmt.notifiers.Len())
			case "subscriptionCount":
				hnd.Val = val.Int32(subscribeCount)
			default:
				return p.Field(r, hnd)
			}
			return nil
		},
	}
}
