package device

import (
	"testing"

	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestMapClient(t *testing.T) {
	ypath := source.Path("./testdata:../yang")
	d := New(ypath)
	d.Add("test", &nodeutil.Basic{})
	dm := NewMap()
	dm.Add("dev0", d)
	dmMod := parser.RequireModule(ypath, "fc-map")
	dmNode := MapNode(dm)
	dmClient := &MapClient{
		proto: func(string) (Device, error) {
			return d, nil
		},
		browser: node.NewBrowser(dmMod, dmNode),
	}
	var gotUpdate bool
	dmClient.OnModuleUpdate("test", func(d Device, id string, c Change) {
		gotUpdate = true
	})
	if !gotUpdate {
		t.Error("never got test message")
	}
}
