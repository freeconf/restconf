package secure

import (
	"github.com/freeconf/yang/val"

	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

func Manage(rbac *Rbac) node.Node {
	return &nodeutil.Node{
		Object: rbac,
		Options: nodeutil.NodeOptions{
			TryPluralOnLists: true,
		},
		OnChild: func(n *nodeutil.Node, r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "authentication":
				return nil, nil
			case "authorization":
				return n, nil
			}
			return n.DoChild(r)
		},
		OnField: func(n *nodeutil.Node, r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "perm":
				ac := n.Object.(*AccessControl)
				if r.Write {
					ac.Permissions = Permission(hnd.Val.Value().(val.Enum).Id)
				} else {
					var err error
					hnd.Val, err = node.NewValue(r.Meta.Type(), ac.Permissions)
					return err
				}
				return nil
			}
			return n.DoField(r, hnd)
		},
	}
}
