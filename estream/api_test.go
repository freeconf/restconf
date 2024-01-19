package estream

import (
	"testing"

	"github.com/freeconf/restconf/testdata"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

func TestApi(t *testing.T) {
	ypath := source.Path("../testdata:../yang/ietf-rfc")
	car := testdata.New()
	carMod := parser.RequireModule(ypath, "car")
	carBwsr := node.NewBrowser(carMod, testdata.Manage(car))

	opts := parser.Options{
		Features: meta.FeaturesOn([]string{
			"replay", "configured", "xpath", "encode-json", "encode-xml",
		}),
	}
	m, err := parser.LoadModuleWithOptions(ypath, "ietf-subscribed-notifications", opts)
	fc.RequireEqual(t, nil, err)
	s := NewService()
	events := make(chan SubEvent, 10)
	s.onEvent(func(e SubEvent) {
		events <- e
	})
	b := node.NewBrowser(m, Manage(s))
	root := b.Root()
	s.AddFilter(Filter{
		Name: "my-filter",
		Filter: func(s *node.Selection) *node.Selection {
			return s
		},
	})
	s.AddStream(Stream{
		Name: "my-stream",
		Open: func() (*node.Selection, error) {
			return carBwsr.Root().Find("update")
		},
	})
	req := `{
		"stream-filter-name" : "my-filter",
		"stream" : "my-stream"
	}`
	rpc := sel(root.Find("establish-subscription"))
	out, err := rpc.Action(readJson(req))
	fc.AssertEqual(t, nil, err)
	actual, err := nodeutil.WriteJSON(out)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, `{"id":"100"}`, actual)
	fc.AssertEqual(t, SubEventStarted, (<-events).EventId)

	badFilter := `{
		"stream-filter-name" : "nope",
		"stream" : "my-stream"
	}`
	_, err = rpc.Action(readJson(badFilter))
	fc.AssertEqual(t, true, err != nil)

	badStream := `{
		"stream-filter-name" : "my-filter",
		"stream" : "nope"
	}`
	_, err = rpc.Action(readJson(badStream))
	fc.AssertEqual(t, true, err != nil)
}

func readJson(s string) node.Node {
	n, err := nodeutil.ReadJSON(s)
	if err != nil {
		panic(err)
	}
	return n
}

func sel(s *node.Selection, err error) *node.Selection {
	if err != nil {
		panic(err)
	}
	return s
}
