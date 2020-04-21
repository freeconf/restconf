package restconf

import (
	"bytes"
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/restconf/secure"
	"github.com/freeconf/restconf/stock"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

type Server struct {
	Web                      *stock.HttpServer
	CallHome                 *CallHome
	Auth                     secure.Auth
	Ver                      string
	NotifyKeepaliveTimeoutMs int
	main                     device.Device
	devices                  device.Map
	notifiers                *list.List
	ypath                    source.Opener

	// Optional: Anything not handled by RESTCONF protocol can call this handler otherwise
	UnhandledRequestHandler http.HandlerFunc
}

func NewServer(d *device.Local) *Server {
	m := &Server{
		notifiers: list.New(),
		ypath:     d.SchemaSource(),
	}
	m.ServeDevice(d)

	if err := d.Add("fc-restconf", Node(m, d.SchemaSource())); err != nil {
		panic(err)
	}

	// Required by all devices according to RFC
	if err := d.Add("ietf-yang-library", device.LocalDeviceYangLibNode(m.ModuleAddress, d)); err != nil {
		panic(err)
	}
	return m
}

func (self *Server) Close() {
	if self.Web != nil {
		self.Web.Server.Close()
		self.Web = nil
	}
}

func (self *Server) ModuleAddress(m *meta.Module) string {
	return fmt.Sprint("schema/", m.Ident(), ".yang")
}

func (self *Server) DeviceAddress(id string, d device.Device) string {
	return fmt.Sprint("/restconf=", id)
}

func (self *Server) ServeDevices(m device.Map) error {
	self.devices = m
	return nil
}

func (self *Server) ServeDevice(d device.Device) error {
	self.main = d
	return nil
}

func (self *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if fc.DebugLogEnabled() {
		fc.Debug.Printf("%s %s", r.Method, r.URL)
		if r.Body != nil {
			content, rerr := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			if rerr != nil {
				fc.Err.Printf("error trying to log body content %s", rerr)
			} else {
				if len(content) > 0 {
					fc.Debug.Print(string(content))
					r.Body = ioutil.NopCloser(bytes.NewBuffer(content))
				}
			}
		}
	}

	h := w.Header()

	// CORS
	h.Set("Access-Control-Allow-Headers", "origin, content-type, accept")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS, DELETE, PATCH")
	h.Set("Access-Control-Allow-Origin", "*")
	if r.URL.Path == "/" {
		switch r.Method {
		case "OPTIONS":
			return
		case "GET":
			http.Redirect(w, r, "restconf/ui/index.html", http.StatusMovedPermanently)
			return
		}
	}

	op1, deviceId, p := shiftOptionalParamWithinSegment(r.URL, '=', '/')
	device, err := self.findDevice(deviceId)
	if err != nil {
		handleErr(err, w)
		return
	}
	switch op1 {
	case ".ver":
		w.Write([]byte(self.Ver))
	case ".well-known":
		self.serveStaticRoute(w, r)
	case "restconf":
		op2, p := shift(p, '/')
		r.URL = p
		switch op2 {
		case "data", "streams":
			self.serveData(device, w, r)
		case "ui":
			self.serveStreamSource(w, device.UiSource(), r.URL.Path)
		case "schema":
			// Hack - parse accept header to get proper content type
			accept := r.Header.Get("Accept")
			fc.Debug.Printf("accept %s", accept)
			if strings.Contains(accept, "/json") {
				self.serveSchema(w, r, device.SchemaSource())
			} else {
				self.serveStreamSource(w, device.SchemaSource(), r.URL.Path)
			}
		default:
			handleErr(badAddressErr, w)
		}
	default:
		if self.UnhandledRequestHandler != nil {
			self.UnhandledRequestHandler(w, r)
			return
		}
	}
}

func (self *Server) serveSchema(w http.ResponseWriter, r *http.Request, ypath source.Opener) {
	modName, p := shift(r.URL, '/')
	r.URL = p
	m, err := parser.LoadModule(ypath, modName)
	if err != nil {
		handleErr(err, w)
		return
	}
	ylib, err := parser.LoadModule(ypath, "fc-yang")
	if err != nil {
		handleErr(err, w)
		return
	}
	b := nodeutil.Schema(ylib, m)
	hndlr := &browserHandler{browser: b}
	hndlr.ServeHTTP(w, r)
}

func (self *Server) serveData(d device.Device, w http.ResponseWriter, r *http.Request) {
	if hndlr, p := self.shiftBrowserHandler(d, w, r.URL); hndlr != nil {
		r.URL = p
		hndlr.ServeHTTP(w, r)
	}
}

func (self *Server) serveStreamSource(w http.ResponseWriter, s source.Opener, path string) {
	rdr, err := s(path, "")
	if err != nil {
		handleErr(err, w)
		return
	} else if rdr == nil {
		handleErr(fc.NotFoundError, w)
		return
	}
	ext := filepath.Ext(path)
	ctype := mime.TypeByExtension(ext)
	w.Header().Set("Content-Type", ctype)
	if _, err := io.Copy(w, rdr); err != nil {
		handleErr(err, w)
	}
}

func (self *Server) findDevice(deviceId string) (device.Device, error) {
	if deviceId == "" {
		return self.main, nil
	}
	device, err := self.devices.Device(deviceId)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("%w. device %s", fc.NotFoundError, deviceId)
	}
	return device, nil
}

func (self *Server) shiftOperationAndDevice(w http.ResponseWriter, orig *url.URL) (string, device.Device, *url.URL) {
	//  operation[=deviceId]/...
	op, deviceId, p := shiftOptionalParamWithinSegment(orig, '=', '/')
	if op == "" {
		handleErr(fmt.Errorf("%w. no operation found in path", fc.NotFoundError), w)
		return op, nil, orig
	}
	device, err := self.findDevice(deviceId)
	if err != nil {
		handleErr(err, w)
		return "", nil, orig
	}
	return op, device, p
}

func (self *Server) shiftBrowserHandler(d device.Device, w http.ResponseWriter, orig *url.URL) (*browserHandler, *url.URL) {
	if module, p := shift(orig, ':'); module != "" {
		if browser, err := d.Browser(module); browser != nil {
			return &browserHandler{
				browser: browser,
			}, p
		} else if err != nil {
			handleErr(err, w)
			return nil, orig
		}
	}

	handleErr(fmt.Errorf("%w. no module found in path", fc.NotFoundError), w)
	return nil, orig
}

func (self *Server) serveStaticRoute(w http.ResponseWriter, r *http.Request) bool {
	_, p := shift(r.URL, '/')
	op, _ := shift(p, '/')
	switch op {
	case "host-meta":
		// RESTCONF Sec. 3.1
		fmt.Fprintf(w, `{ "xrd" : { "link" : { "@rel" : "restconf", "@href" : "/restconf" } } }`)
		return true
	}
	return false
}
