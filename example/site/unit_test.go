package site

import (
	"testing"

	"github.com/freeconf/restconf/testdata"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestUnitTestingPartialYang(t *testing.T) {

	ypath := source.Dir("../../testdata")
	m, err := parser.LoadModuleFromString(ypath, `
module x {
	import car {
		prefix "c";
	}

	uses c:tire;
}
	`)
	fc.AssertEqual(t, nil, err)
	tire := &testdata.Tire{Pos: 10, Size: "x"}
	b := node.NewBrowser(m, testdata.TireNode(tire))
	actual, err := nodeutil.WriteJSON(b.Root())
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, `{"pos":10,"size":"x","worn":true,"wear":0,"flat":false}`, actual)
}
