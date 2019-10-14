package device

import (
	"testing"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestMapNode(t *testing.T) {
	ypath := source.Path("./testdata:../yang")
	d := New(ypath)
	d.Add("test", &nodeutil.Basic{})
	dm := NewMap()
	dm.Add("dev0", d)
	dmMod := parser.RequireModule(ypath, "fc-map")
	dmNode := MapNode(dm)
	b := node.NewBrowser(dmMod, dmNode)
	actual, err := nodeutil.WriteJSON(b.Root().Find("device=dev0"))
	if err != nil {
		t.Error(err)
	}
	expected := `{"deviceId":"dev0","module":[{"name":"test","revision":"0"}]}`
	fc.AssertEqual(t, expected, actual)
}
