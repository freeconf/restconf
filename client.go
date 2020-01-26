package restconf

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

// NewClient interfaces with a remote RESTCONF server.  This also implements device.Device
// making it appear like a local device and is important architecturaly.  Code that uses
// this in a node.Browser context would not know the difference from a remote or local device
// with one minor exceptions. Peek() wouldn't work.
type Client struct {
	YangPath source.Opener
}

func ProtocolHandler(ypath source.Opener) device.ProtocolHandler {
	c := Client{YangPath: ypath}
	return c.NewDevice
}

type Address struct {
	Base     string
	Data     string
	Stream   string
	Ui       string
	Schema   string
	DeviceId string
	Host     string
	Origin   string
}

func NewAddress(urlAddr string) (Address, error) {
	// remove trailing '/' if there is one to prepare for appending
	if urlAddr[len(urlAddr)-1] != '/' {
		urlAddr = urlAddr + "/"
	}

	urlParts, err := url.Parse(urlAddr)
	if err != nil {
		return Address{}, err
	}

	return Address{
		Base:     urlAddr,
		Data:     urlAddr + "data/",
		Schema:   urlAddr + "schema/",
		Ui:       urlAddr + "ui/",
		Origin:   "http://" + urlParts.Host,
		DeviceId: findDeviceIdInUrl(urlAddr),
	}, nil
}

func findDeviceIdInUrl(addr string) string {
	segs := strings.SplitAfter(addr, "/restconf=")
	if len(segs) == 2 {
		post := segs[1]
		return post[:len(post)-1]
	}
	return ""
}

func (self Client) NewDevice(url string) (device.Device, error) {
	address, err := NewAddress(url)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	remoteSchemaPath := httpStream{
		ypath:  self.YangPath,
		client: httpClient,
		url:    address.Schema,
	}
	c := &client{
		address:    address,
		yangPath:   self.YangPath,
		schemaPath: source.Any(self.YangPath, remoteSchemaPath.OpenStream),
		client:     httpClient,
	}
	d := &clientNode{support: c, device: address.DeviceId}
	m := parser.RequireModule(self.YangPath, "ietf-yang-library")
	b := node.NewBrowser(m, d.node())
	modules, err := device.LoadModules(b, remoteSchemaPath)
	fc.Debug.Printf("loaded modules %v", modules)
	if err != nil {
		return nil, fmt.Errorf("could not load modules. %s", err)
	}
	c.modules = modules
	return c, nil
}

var badAddressErr = errors.New("Expected format: http://server/restconf[=device]/operation/module:path")

type client struct {
	address    Address
	yangPath   source.Opener
	schemaPath source.Opener
	client     *http.Client
	origin     string
	modules    map[string]*meta.Module
}

func (self *client) SchemaSource() source.Opener {
	return self.schemaPath
}

func (self *client) UiSource() source.Opener {
	s := httpStream{
		client: self.client,
		url:    self.address.Ui,
	}
	return s.OpenStream
}

func (self *client) Browser(module string) (*node.Browser, error) {
	d := &clientNode{support: self, device: self.address.DeviceId}
	m, err := self.module(module)
	if err != nil {
		return nil, err
	}
	return node.NewBrowser(m, d.node()), nil
}

func (self *client) Close() {
}

func (self *client) Modules() map[string]*meta.Module {
	return self.modules
}

func (self *client) module(module string) (*meta.Module, error) {
	// caching module, but should replace w/cache that can refresh on stale
	m := self.modules[module]
	if m == nil {
		var err error
		if m, err = parser.LoadModule(self.schemaPath, module); err != nil {
			return nil, err
		}
		self.modules[module] = m
	}
	return m, nil
}

func (self *client) clientStream(params string, p *node.Path, ctx context.Context) (<-chan node.Node, error) {
	mod := meta.RootModule(p.Meta())
	fullUrl := fmt.Sprint(self.address.Data, mod.Ident(), ":", p.StringNoModule())
	req, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	fc.Info.Printf("<=> SSE %s", fullUrl)
	resp, err := self.client.Do(req)
	if err != nil {
		return nil, err
	}
	stream := make(chan node.Node)
	go func() {
		events := decodeSse(resp.Body)
		defer resp.Body.Close()
		for {
			select {
			case event := <-events:
				stream <- nodeutil.ReadJSONIO(bytes.NewReader(event))
			case <-ctx.Done():
				return
			}
		}
	}()

	return stream, nil
}

// ClientSchema downloads schema and implements yang.StreamSource so it can transparently
// be used in a YangPath.
type httpStream struct {
	ypath  source.Opener
	client *http.Client
	url    string
}

func (self httpStream) ResolveModuleHnd(hnd device.ModuleHnd) (*meta.Module, error) {
	m, _ := parser.LoadModule(self.ypath, hnd.Name)
	if m != nil {
		return m, nil
	}
	return parser.LoadModule(self.OpenStream, hnd.Name)
}

// OpenStream implements source.Opener
func (self httpStream) OpenStream(name string, ext string) (io.Reader, error) {
	fullUrl := self.url + name + ext
	fc.Debug.Printf("httpStream url %s, name=%s, ext=%s", fullUrl, name, ext)
	resp, err := self.client.Get(fullUrl)
	if resp != nil {
		return resp.Body, err
	}
	return nil, err
}

func (self *client) clientDo(method string, params string, p *node.Path, payload io.Reader) (node.Node, error) {
	var req *http.Request
	var err error
	mod := meta.RootModule(p.Meta())
	fullUrl := fmt.Sprint(self.address.Data, mod.Ident(), ":", p.StringNoModule())
	if params != "" {
		fullUrl = fmt.Sprint(fullUrl, "?", params)
	}
	if req, err = http.NewRequest(method, fullUrl, payload); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	fc.Info.Printf("=> %s %s", method, fullUrl)
	resp, getErr := self.client.Do(req)
	if getErr != nil || resp.Body == nil {
		return nil, getErr
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("(%d) %s", resp.StatusCode, string(msg))
	}
	return nodeutil.ReadJSONIO(resp.Body), nil
}
