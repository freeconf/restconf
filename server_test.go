package restconf

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/freeconf/yang/fc"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/restconf/testdata"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

var updateFlag = flag.Bool("update", false, "update golden files instead of verifying against them")

func TestServer(t *testing.T) {
	ypath := source.Path("./testdata:./yang")
	m := parser.RequireModule(ypath, "car")
	car := testdata.New()
	bServer := node.NewBrowser(m, testdata.Manage(car))
	d := device.New(ypath)
	d.AddBrowser(bServer)
	s := NewServer(d)
	defer s.Close()
	err := d.ApplyStartupConfig(strings.NewReader(`
		{
			"fc-restconf" : {
				"web": {
					"port" : ":9080"
				},
				"debug" : true
			}
		}`))
	if err != nil {
		t.Fatal(err)
	}
	<-time.After(2 * time.Second)

	client := http.DefaultClient
	addr := "http://localhost:9080"

	r, err := client.Get(addr + "/.well-known/host-meta")
	goldResponse(t, "testdata/gold/well-known", r, err)

	r, err = client.Get(addr + "/restconf/schema/car.yang")
	goldResponse(t, "testdata/gold/car.yang", r, err)

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/restconf/schema/car.yang", addr), nil)
	req.Header.Set("Accept-Type", "application/json")
	r, err = client.Do(req)
	goldResponse(t, "testdata/gold/car.json", r, err)

	r, _ = client.Get(addr + "/restconf/schema/bogus")
	if r.StatusCode != 404 {
		t.Errorf("expected 404 got %d", r.StatusCode)
	}
	contentType := "application/json"

	r, _ = client.Post(addr+"/restconf/operations/car:rotateTires", contentType, nil)
	fc.AssertEqual(t, 200, r.StatusCode)

	r, _ = client.Post(addr+"/restconf/data/car:rotateTires", YangDataJsonMimeType, nil)
	fc.AssertEqual(t, 400, r.StatusCode)

	r, _ = client.Post(addr+"/restconf/data/car:rotateTires", contentType, nil)
	fc.AssertEqual(t, 200, r.StatusCode)

	s.Close()
}

func goldResponse(t *testing.T, goldFile string, r *http.Response, err error) {
	t.Helper()
	if err != nil {
		t.Error(err)
		return
	}
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
		return
	}
	fc.Gold(t, *updateFlag, data, goldFile)
	if r.StatusCode != 200 {
		t.Errorf("gave status code %d", r.StatusCode)
	}
}
