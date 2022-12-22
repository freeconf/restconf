package restconf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/restconf/testdata"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
)

func TestClient(t *testing.T) {
	// setup a server on a port and use client to connect
	ypath := source.Path("./yang:./testdata")
	car := testdata.New()
	local := device.New(ypath)
	local.Add("car", testdata.Manage(car))
	/*server := */ NewServer(local)
	cfg := `{
		"fc-restconf": {
			"web" : {
				"port": ":10999"
			}
		},
		"car" : {
			"speed": 5
		}
	}`
	fc.RequireEqual(t, nil, local.ApplyStartupConfig(strings.NewReader(cfg)))

	c := Client{YangPath: ypath}
	dev, err := c.NewDevice("http://localhost:10999/restconf")
	fc.RequireEqual(t, nil, err)
	b, err := dev.Browser("car")
	fc.RequireEqual(t, nil, err)

	root := b.Root()

	// read
	actual, err := nodeutil.WritePrettyJSON(root.Constrain("content=config"))
	fc.AssertEqual(t, nil, err)
	fc.Gold(t, *updateFlag, []byte(actual), "testdata/gold/client-read.json")

	// test
	fc.AssertEqual(t, false, root.Find("tire=0").IsNil())
	fc.AssertEqual(t, true, root.Find("tire=99").IsNil())

	// rpc
	before := car.Tire[0].Wear
	fc.AssertEqual(t, nil, root.Find("replaceTires").Action(nil).LastErr)
	after := car.Tire[0].Wear
	fc.AssertEqual(t, false, before > after, fmt.Sprintf("%f > %f", before, after))

	// notify
	done := make(chan bool)
	sub, err := root.Find("update?filter=running%3D'false'").Notifications(func(n node.Notification) {
		done <- true
	})
	fc.RequireEqual(t, nil, err)
	<-done
	sub()
}
