package client

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestClientNotif(t *testing.T) {
	ypath := source.Path("../testdata:../yang")
	m := parser.RequireModule(ypath, "x")
	var s *restconf.Server
	send := make(chan string)
	n := &nodeutil.Basic{
		OnNotify: func(r node.NotifyRequest) (node.NotifyCloser, error) {
			go func() {
				s := <-send
				fmt.Println("sending message")
				r.Send(nodeutil.ReflectChild(map[string]interface{}{
					"z": s,
				}))
			}()
			return func() error {
				return nil
			}, nil
		},
	}
	bServer := node.NewBrowser(m, n)
	d := device.New(ypath)
	d.AddBrowser(bServer)
	s = restconf.NewServer(d)
	defer s.Close()
	err := d.ApplyStartupConfig(strings.NewReader(`
		{
			"fc-restconf" : {
				"web": {
					"port" : ":9081"
				},
				"debug" : true
			}
		}`))
	if err != nil {
		t.Fatal(err)
	}
	// wait for server to startup
	<-time.After(2 * time.Second)

	recv := make(chan string)

	testClient := func(compliance restconf.ComplianceOptions) error {
		factory := Client{YangPath: ypath, Complance: compliance}
		dev, err := factory.NewDevice("http://localhost:9081/restconf")
		if err != nil {
			return err
		}
		b, err := dev.Browser("x")
		if err != nil {
			return err
		}
		sub, err := sel(b.Root().Find("y")).Notifications(func(msg node.Notification) {
			fmt.Println("receiving message")
			actual, err := nodeutil.WriteJSON(msg.Event)
			if err != nil {
				panic(err)
			}
			recv <- actual
		})
		if err != nil {
			return err
		}
		send <- "original session"
		actual := <-recv
		if actual != `{"z":"original session"}` {
			return fmt.Errorf("not expected output %s", actual)
		}
		sub()
		return nil
	}

	fc.AssertEqual(t, nil, testClient(restconf.Simplified))
	fc.AssertEqual(t, nil, testClient(restconf.Strict))
}
