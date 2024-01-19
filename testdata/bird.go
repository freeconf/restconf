package testdata

import (
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

type Bird struct {
	Name     string
	Wingspan int
	Species  *Species
}

type Species struct {
	Name  string
	Class string
}

var YangPath = source.Path("../testdata:../yang")

func BirdDevice(json string) (*device.Local, map[string]*Bird) {
	d := device.New(YangPath)
	b, birds := BirdBrowser(json)
	d.AddBrowser(b)
	if json != "" {
		if err := b.Root().UpsertFrom(readJson(json)); err != nil {
			panic(err)
		}
	}
	return d, birds
}

// not useful in prod
func readJson(s string) node.Node {
	n, err := nodeutil.ReadJSON(s)
	if err != nil {
		panic(err)
	}
	return n
}

func BirdBrowser(json string) (*node.Browser, map[string]*Bird) {
	data := make(map[string]*Bird)
	b := node.NewBrowser(BirdModule(), BirdNode(data))
	if json != "" {
		if err := b.Root().UpsertFrom(readJson(json)); err != nil {
			panic(err)
		}
	}
	return b, data
}

func BirdModule() *meta.Module {
	return parser.RequireModule(YangPath, "bird")
}

func BirdNode(birds map[string]*Bird) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "bird":
				return nodeutil.ReflectList(birds), nil
			}
			return nil, nil
		},
	}
}
