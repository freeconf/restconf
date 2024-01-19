package estream

import (
	"testing"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
)

var mstr = `
module x {
	notification msgs {
		leaf msg {
			type string;
		}
	}
}
`

func TestSubReceiver(t *testing.T) {
	type msg struct {
		Msg string
	}
	msgs := make(chan msg)
	n := &nodeutil.Basic{
		OnNotify: func(r node.NotifyRequest) (node.NotifyCloser, error) {
			closer := func() error {
				close(msgs)
				return nil
			}
			go func() {
				for m := range msgs {
					r.Send(&nodeutil.Node{Object: &m})
				}
			}()
			return closer, nil
		},
	}
	m, err := parser.LoadModuleFromString(nil, mstr)
	fc.RequireEqual(t, nil, err)
	b := node.NewBrowser(m, n)

	s := NewSubscription("X", nil)
	opts := SubscriptionOptions{
		Stream: Stream{
			Name: "foo",
			Open: func() (*node.Selection, error) {
				return b.Root().Find("msgs")
			},
		},
	}
	err = s.Apply(opts)
	fc.RequireEqual(t, nil, err)
	recvr := make(chan ReceiverEvent)
	err = s.AddReceiver("foo", func(e ReceiverEvent) error {
		recvr <- e
		return nil
	})
	fc.RequireEqual(t, nil, err)
	msgs <- msg{Msg: "hello"}
	actual, err := nodeutil.WriteJSON((<-recvr).Event)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, `{"msg":"hello"}`, actual)
	s.RemoveReceiver("foo")
}
