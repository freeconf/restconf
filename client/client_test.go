package client

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/restconf/testdata"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
)

var updateFlag = flag.Bool("update", false, "update golden files instead of verifying against them")

func TestClient(t *testing.T) {
	// setup a server on a port and use client to connect
	ypath := source.Path("../yang:../testdata")
	car := testdata.New()
	local := device.New(ypath)
	local.Add("car", testdata.Manage(car))
	s := restconf.NewServer(local)
	defer s.Close()
	cfg := `{
		"fc-restconf": {
			"debug": true,
			"web" : {
				"port": ":10999"
			}
		},
		"car" : {
			"speed": 5
		}
	}`
	fc.RequireEqual(t, nil, local.ApplyStartupConfig(strings.NewReader(cfg)))

	testClient := func(compliance restconf.ComplianceOptions) error {
		t.Logf("compliance %s", compliance)
		c := Client{YangPath: ypath, Complance: compliance}
		dev, err := c.NewDevice("http://localhost:10999/restconf")
		fc.RequireEqual(t, nil, err)
		b, err := dev.Browser("car")
		fc.RequireEqual(t, nil, err)

		root := b.Root()

		// read
		actual, err := nodeutil.WritePrettyJSON(sel(root.Constrain("content=config")))
		fc.AssertEqual(t, nil, err)
		fc.Gold(t, *updateFlag, []byte(actual), "testdata/gold/client-read.json")

		// test
		fc.AssertEqual(t, true, sel(root.Find("tire=0")) != nil)
		tireSel, _ := root.Find("tire=99")
		fc.AssertEqual(t, true, tireSel == nil)

		// rpc
		before := car.Tire[0].Wear
		sel(sel(root.Find("replaceTires")).Action(nil))
		after := car.Tire[0].Wear
		fc.AssertEqual(t, false, before > after, fmt.Sprintf("%f > %f", before, after))

		// notify
		done := make(chan bool)
		sub, err := sel(root.Find("update?filter=running%3D'false'")).Notifications(func(n node.Notification) {
			done <- true
		})
		fc.RequireEqual(t, nil, err)
		<-done
		sub()
		return nil
	}
	testClient(restconf.Simplified)
	testClient(restconf.Strict)
}

func sel(sel *node.Selection, err error) *node.Selection {
	if err != nil {
		panic(err)
	}
	return sel
}
