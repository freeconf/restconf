package restconf

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
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
	webApps                  []webApp
	Auth                     secure.Auth
	Ver                      string
	NotifyKeepaliveTimeoutMs int
	main                     device.Device
	devices                  device.Map
	notifiers                *list.List
	ypath                    source.Opener

	// Optional: Anything not handled by RESTCONF protocol can call this handler otherwise
	UnhandledRequestHandler http.HandlerFunc

	// Give app change to read custom header data and stuff into context so info can get
	// to app layer
	Filters []RequestFilter
}

type RequestFilter func(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error)

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
	ctx := context.Background()
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
	for _, f := range self.Filters {
		var err error
		if ctx, err = f(ctx, w, r); err != nil {
			handleErr(err, w)
			return
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
			http.Redirect(w, r, "app/", http.StatusMovedPermanently)
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
		return
	case ".well-known":
		self.serveStaticRoute(w, r)
		return
	case "restconf":
		op2, p := shift(p, '/')
		r.URL = p
		switch op2 {
		case "data", "streams":
			self.serveData(ctx, device, w, r)
		case "ui":
			self.serveStreamSource(w, device.UiSource(), r.URL.Path)
		case "schema":
			// Hack - parse accept header to get proper content type
			accept := r.Header.Get("Accept")
			fc.Debug.Printf("accept %s", accept)
			if strings.Contains(accept, "/json") {
				self.serveSchema(ctx, w, r, device.SchemaSource())
			} else {
				self.serveStreamSource(w, device.SchemaSource(), r.URL.Path)
			}
		default:
			handleErr(badAddressErr, w)
		}
		return
	}
	if self.handleWebApp(w, r, op1, p.Path) {
		return
	}
	if self.UnhandledRequestHandler != nil {
		self.UnhandledRequestHandler(w, r)
		return
	}
}

func (self *Server) serveSchema(ctx context.Context, w http.ResponseWriter, r *http.Request, ypath source.Opener) {
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
	hndlr.ServeHTTP(ctx, w, r)
}

func (self *Server) serveData(ctx context.Context, d device.Device, w http.ResponseWriter, r *http.Request) {
	if hndlr, p := self.shiftBrowserHandler(d, w, r.URL); hndlr != nil {
		r.URL = p
		hndlr.ServeHTTP(ctx, w, r)
	}
}

type webApp struct {
	endpoint string
	homeDir  string
	homePage string
}

func (self *Server) RegisterWebApp(homeDir string, homePage string, endpoint string) {
	self.webApps = append(self.webApps, webApp{
		endpoint: endpoint,
		homeDir:  homeDir,
		homePage: homePage,
	})
}

// Serve web app according to SPA conventions where you serve static assets if
// they exist but if they don't assume, the URL is going to be interpretted
// in browser as route path.
func (self *Server) handleWebApp(w http.ResponseWriter, r *http.Request, endpoint string, path string) bool {
	for _, wap := range self.webApps {

		if endpoint == wap.endpoint {

			// if someone type "/app/index.html" then direct them to right spot
			if strings.HasPrefix(path, wap.homePage) {
				// redirect to root path so URL is correct in browser
				http.Redirect(w, r, wap.endpoint, http.StatusMovedPermanently)
				return true
			}

			// redirect to root path so URL is correct in browser
			self.serveWebApp(w, r, wap, path)
			return true
		}
	}
	return false
}

func (self *Server) serveWebApp(w http.ResponseWriter, r *http.Request, wap webApp, path string) {
	var rdr *os.File
	useHomePage := false
	if path == "" {
		useHomePage = true
	} else {
		var ferr error
		rdr, ferr = os.Open(filepath.Join(wap.homeDir, path))
		if ferr != nil {
			if os.IsNotExist(ferr) {
				useHomePage = true
			} else {
				handleErr(ferr, w)
				return
			}
		} else {
			// If you do not find a file, assume it's a path that resolves
			// in client and we send the home page.
			stat, _ := rdr.Stat()
			useHomePage = stat.IsDir()
		}
	}
	var ext string
	if useHomePage {
		var ferr error
		rdr, ferr = os.Open(filepath.Join(wap.homeDir, wap.homePage))
		if ferr != nil {
			if os.IsNotExist(ferr) {
				handleErr(fc.NotFoundError, w)
			} else {
				handleErr(ferr, w)
			}
			return
		}
		ext = ".html"
	} else {
		ext = filepath.Ext(path)
	}
	ctype := mime.TypeByExtension(ext)
	w.Header().Set("Content-Type", ctype)
	if _, err := io.Copy(w, rdr); err != nil {
		handleErr(err, w)
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
