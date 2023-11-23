package restconf

import (
	"flag"
	"io"
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

	r, _ = client.Get(addr + "/restconf/schema/bogus")
	if r.StatusCode != 404 {
		t.Errorf("expected 404 got %d", r.StatusCode)
	}

	rfcMimes := []MimeType{
		YangDataJsonMimeType1, YangDataJsonMimeType2,
		YangDataXmlMimeType1, YangDataXmlMimeType2,
	}

	t.Run("rpc", func(t *testing.T) {
		// weird + strict RFC Complianiance = OK
		for _, c := range rfcMimes {
			r, _ = client.Post(addr+"/restconf/operations/car:rotateTires", string(c), nil)
			fc.AssertEqual(t, 204, r.StatusCode, string(c))
		}

		// weird + relaxed RFC Complianiance = OK
		r, _ = client.Post(addr+"/restconf/operations/car:rotateTires", string(PlainJsonMimeType), nil)
		fc.AssertEqual(t, 204, r.StatusCode)

		// intuative + strict RFC Complianiance = NOT OK
		for _, c := range rfcMimes {
			r, _ = client.Post(addr+"/restconf/data/car:rotateTires", string(c), nil)
			fc.AssertEqual(t, 400, r.StatusCode, string(c))
		}

		// intuative + relaxed RFC Complianiance  = OK
		r, _ = client.Post(addr+"/restconf/data/car:rotateTires", string(PlainJsonMimeType), nil)
		fc.AssertEqual(t, 204, r.StatusCode)
	})

	t.Run("rpc-io", func(t *testing.T) {
		tests := []struct {
			format MimeType
			input  string
			output string
		}{
			{
				format: PlainJsonMimeType,
				input:  `{"source":"tripa"}`,
				output: `{"miles":0}`,
			},
			{
				format: YangDataJsonMimeType1,
				input:  `{"car:input":{"source":"tripa"}}`,
				output: `{"car:output":{"miles":0}}`,
			},
			{
				format: YangDataXmlMimeType1,
				input:  `<input xmlns="c"><source>tripa</source></input>`,
				output: `<output xmlns="c"><miles>0</miles></output>`,
			},
		}
		for _, test := range tests {
			payload := strings.NewReader(test.input)
			req, err := http.NewRequest("POST", addr+"/restconf/operations/car:getMiles", payload)
			fc.RequireEqual(t, nil, err)
			req.Header.Set("Content-Type", string(test.format))
			req.Header.Set("Accept", string(test.format))
			resp, err := client.Do(req)
			fc.RequireEqual(t, nil, err)
			fc.AssertEqual(t, 200, resp.StatusCode)
			actual, err := io.ReadAll(resp.Body)
			fc.RequireEqual(t, nil, err)
			fc.AssertEqual(t, test.output, string(actual))
		}
	})

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
