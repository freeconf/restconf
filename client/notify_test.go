package client

import (
	"strings"
	"testing"
	"time"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestClientNotif(t *testing.T) {
	ypath := source.Path("../testdata:../yang")
	m := parser.RequireModule(ypath, "x")
	var s *restconf.Server
	send := make(chan string, 1)
	n := &nodeutil.Basic{
		OnNotify: func(r node.NotifyRequest) (node.NotifyCloser, error) {
			go func() {
				for s := range send {
					r.Send(nodeutil.ReflectChild(map[string]interface{}{
						"z": s,
					}))
				}
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
	<-time.After(2 * time.Second)
	factory := Client{YangPath: ypath}
	c, err := factory.NewDevice("http://localhost:9081/restconf")
	if err != nil {
		t.Fatal(err)
	}
	bClient, err := c.Browser("x")
	if err != nil {
		t.Fatal(err)
	}
	send <- "original session"
	recv := make(chan string, 1)
	sub, err := bClient.Root().Find("y").Notifications(func(msg node.Notification) {
		actual, err := nodeutil.WriteJSON(msg.Event)
		if err != nil {
			t.Fatal(err)
		}
		recv <- actual
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := <-recv
	if msg != `{"x:z":"original session"}` {
		t.Error(msg)
	}
	sub()
	s.Close()
}
