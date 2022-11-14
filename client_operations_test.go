package restconf

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
	support := &testDriverFlowSupport{
		t: t,
	}
	expected := `{"mileage":{"odometer":1000}}`
	support.get = map[string]string{
		"car": expected,
	}
	d := &clientNode{support: support}
	b := node.NewBrowser(m, d.node())
	if actual, err := nodeutil.WriteJSON(b.Root().Find("car")); err != nil {
		t.Error(err)
	} else {
		fc.AssertEqual(t, expected, actual)
	}

	support.get = map[string]string{
		"car": `{}`,
	}
	expectedEdit := `{"mileage":{"odometer":1001}}`
	edit := nodeutil.ReadJSON(expectedEdit)
	if err := b.Root().Find("car").UpsertFrom(edit).LastErr; err != nil {
		t.Error(err)
	}
	fc.AssertEqual(t, expectedEdit, support.put["car"])
}

type testDriverFlowSupport struct {
	t    *testing.T
	get  map[string]string
	put  map[string]string
	post map[string]string
}

func (self *testDriverFlowSupport) clientDo(method string, params string, p *node.Path, payload io.Reader) (io.ReadCloser, error) {
	path := p.StringNoModule()
	switch method {
	case "GET":
		in, found := self.get[path]
		if !found {
			return nil, fmt.Errorf("no response for %s", path)
		}
		return io.NopCloser(strings.NewReader(in)), nil
	case "PUT":
		body, _ := ioutil.ReadAll(payload)
		self.put = map[string]string{
			path: string(body),
		}
	case "POST":
		body, _ := ioutil.ReadAll(payload)
		self.post = map[string]string{
			path: string(body),
		}
	}
	return nil, nil
}

func (self *testDriverFlowSupport) clientStream(params string, p *node.Path, ctx context.Context) (<-chan streamEvent, error) {
	panic("not implemented")
}
