package client

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"io/ioutil"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
)

func Test_ClientOperations(t *testing.T) {
	m, err := parser.LoadModuleFromString(nil, `module x {namespace ""; prefix ""; revision 0;
		container car {			
			container mileage {
				leaf odometer {
					type int32;
				}
				leaf trip {
					type int32;
				}
			}		
			container make {
				leaf model {
					type string;
				}
			}	
		}
}`)
	if err != nil {
		t.Fatal(err)
	}
	support := newTestDriverFlowSupport(t)
	expected := `{"mileage":{"odometer":1000}}`
	support.get = map[string]string{
		"car": expected,
	}
	d := &clientNode{support: support}
	b := node.NewBrowser(m, d.node())
	if actual, err := nodeutil.WriteJSON(sel(b.Root().Find("car"))); err != nil {
		t.Error(err)
	} else {
		fc.AssertEqual(t, expected, actual)
	}

	support.get = map[string]string{
		"car": `{}`,
	}
	expectedEdit := `{"mileage":{"odometer":1001}}`
	edit := nodeutil.ReadJSON(expectedEdit)
	fc.RequireEqual(t, nil, sel(b.Root().Find("car")).UpsertFrom(edit))
	fc.AssertEqual(t, expectedEdit, support.patch["car"])
}

type testDriverFlowSupport struct {
	t       *testing.T
	get     map[string]string
	put     map[string]string
	post    map[string]string
	patch   map[string]string
	options map[string]string
}

func newTestDriverFlowSupport(t *testing.T) *testDriverFlowSupport {
	return &testDriverFlowSupport{
		t:       t,
		put:     make(map[string]string),
		post:    make(map[string]string),
		patch:   make(map[string]string),
		options: make(map[string]string),
	}
}

func (self *testDriverFlowSupport) clientDo(method string, params string, p *node.Path, payload io.Reader) (io.ReadCloser, error) {
	path := p.StringNoModule()
	var to map[string]string
	switch method {
	case "GET":
		in, found := self.get[path]
		if !found {
			return nil, fmt.Errorf("no response for %s", path)
		}
		return io.NopCloser(strings.NewReader(in)), nil
	case "PUT":
		to = self.put
	case "POST":
		to = self.post
	case "PATCH":
		to = self.patch
	case "OPTIONS":
		to = self.options
	}

	var body string
	if payload != nil {
		data, _ := ioutil.ReadAll(payload)
		body = string(data)
	}
	to[path] = string(body)
	return nil, nil
}

func (self *testDriverFlowSupport) clientStream(params string, p *node.Path, ctx context.Context) (<-chan streamEvent, error) {
	panic("not implemented")
}
