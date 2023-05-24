package secure

import (
	"testing"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/source"

	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
)

func TestManage(t *testing.T) {
	a := NewRbac()
	ypath := source.Dir("../yang")
	b := node.NewBrowser(parser.RequireModule(ypath, "fc-secure"), Manage(a))
	err := b.Root().UpsertFrom(nodeutil.ReadJSON(`{
		"authorization" : {
			"role" : [{
				"id" : "sales",
				"access" : [{
					"path" : "m",
					"perm" : "read"
				},{
					"path" : "m/x",
					"perm" : "none"
				},{
					"path" : "m/z",
					"perm" : "full"				
				}]			
			}]	
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	fc.AssertEqual(t, 1, len(a.Roles))
	//fc.AssertEqual(t, 3, len(a.Roles["sales"].Access))
}
