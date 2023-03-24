package site

import (
	"testing"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/val"
)

type Coordinates struct{}

func (Coordinates) Set(string) {
}

func (Coordinates) Get() string {
	return "0,0"
}

type Bird struct {
	Name     string
	Location Coordinates
}

func TestExtendSimple(t *testing.T) {
	//////////////////////////////
	// BEGIN DOC EXAMPLE CODE
	//////////////////////////////

	modStr := `
module anyName {
	leaf name {
		type string;
	}
	leaf location {
		type string {
			// latitude,longitude (DD)
			pattern "[0-9.]+,[0-9.]+";			
		}
	}
}	
	`
	bird := &Bird{}
	manage := &nodeutil.Extend{
		Base: nodeutil.ReflectChild(bird), // handles Name
		OnField: func(p node.Node, r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "location":
				if r.Write {
					bird.Location.Set(hnd.Val.String())
				} else {
					hnd.Val = val.String(bird.Location.Get())
				}
			default:
				// delegates to ReflectChild
				return p.Field(r, hnd)
			}
			return nil
		},
	}

	//////////////////////////////
	// END DOC EXAMPLE CODE
	//////////////////////////////

	mod, err := parser.LoadModuleFromString(nil, modStr)
	fc.AssertEqual(t, nil, err)
	b := node.NewBrowser(mod, manage)
	root := b.Root()
	actual, err := nodeutil.WriteJSON(root)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, `{"anyName:location":"0,0"}`, actual)
}
