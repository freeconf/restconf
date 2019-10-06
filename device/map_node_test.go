package device

import (
	"testing"

	"github.com/freeconf/yang/c2"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodes"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestMapNode(t *testing.T) {
	ypath := source.Path("./testdata:../yang")
	d := New(ypath)
	d.Add("test", &nodes.Basic{})
	dm := NewMap()
	dm.Add("dev0", d)
	dmMod := parser.RequireModule(ypath, "fc-map")
	dmNode := MapNode(dm)
	b := node.NewBrowser(dmMod, dmNode)
	actual, err := nodes.WriteJSON(b.Root().Find("device=dev0"))
	if err != nil {
		t.Error(err)
	}
	expected := `{"deviceId":"dev0","module":[{"name":"test","revision":"0"}]}`
	c2.AssertEqual(t, expected, actual)
}
