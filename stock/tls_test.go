package stock

import (
	"testing"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestTlsNode(t *testing.T) {

	// where car.yang is stored
	ypath := source.Dir("../yang")

	// Define new YANG module on the fly that references the application
	// YANG file but we pull in just what we want
	m, err := parser.LoadModuleFromString(ypath, `
		module x {
			import fc-stocklib {
				prefix "x";
			}
			uses x:tls;
		}
	`)
	fc.RequireEqual(t, nil, err)
	scfg := `{
		"cert":{
			"certFile": "testdata/test.crt",
			"keyFile": "testdata/test.key"
		}
	}`
	cfg := &Tls{}
	b := node.NewBrowser(m, TlsNode(cfg))
	fc.AssertEqual(t, nil, b.Root().UpsertFrom(nodeutil.ReadJSON(scfg)))
	actual, err := nodeutil.WriteJSON(b.Root())
	fc.RequireEqual(t, nil, err)
	fc.RequireEqual(t, `{"cert":{"certFile":"testdata/test.crt","keyFile":"testdata/test.key"}}`, actual)
}
