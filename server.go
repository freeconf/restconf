package restconf

import (
	"bytes"
	"container/list"
	"context"
	"errors"
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

	// allow rpc to serve under /restconf/data/{module:}/{rpc} which while intuative and
	// original design, it is not in compliance w/RESTCONF spec
	OnlyStrictCompliance bool
}

var ErrBadAddress = errors.New("expected format: http://server/restconf[=device]/operation/module:path")

type RequestFilter func(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error)

func NewServer(d *device.Local) *Server {
	m := NewHttpServe(d)
	if err := d.Add("fc-restconf", Node(m, d.SchemaSource())); err != nil {
		panic(err)
	}
	return m
}

func NewHttpServe(d *device.Local) *Server {
	m := &Server{
		notifiers: list.New(),
		ypath:     d.SchemaSource(),
	}
	m.ServeDevice(d)

	// Required by all devices according to RFC
	if err := d.Add("ietf-yang-library", device.LocalDeviceYangLibNode(m.ModuleAddress, d)); err != nil {
		panic(err)
	}
	return m
}

func (srv *Server) Close() error {
	if srv.Web == nil {
		return nil
	}
	err := srv.Web.Server.Close()
	srv.Web = nil
	return err
}

func (srv *Server) ModuleAddress(m *meta.Module) string {
	return fmt.Sprint("schema/", m.Ident(), ".yang")
}

func (srv *Server) DeviceAddress(id string, d device.Device) string {
	return fmt.Sprint("/restconf=", id)
}

func (srv *Server) ServeDevices(m device.Map) error {
	srv.devices = m
	return nil
}

func (srv *Server) ServeDevice(d device.Device) error {
	srv.main = d
	return nil
}

func (srv *Server) determineCompliance(r *http.Request, contentType MimeType, acceptType MimeType) ComplianceOptions {
	if srv.OnlyStrictCompliance {
		return Strict
	}
	if r.URL.Query().Has(SimplifiedComplianceParam) {
		return Simplified
	}
	if contentType.IsRfc() || acceptType.IsRfc() {
		return Strict
	}
	if acceptType == TextStreamMimeType {
		return Strict
	}
	return Simplified
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := MimeType(r.Header.Get("Content-Type"))
	acceptType := MimeType(r.Header.Get("Accept"))
	compliance := srv.determineCompliance(r, contentType, acceptType)
	fc.Debug.Printf("compliance %s", compliance)
	ctx := context.WithValue(r.Context(), ComplianceContextKey, compliance)
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
	for _, f := range srv.Filters {
		var err error
		if ctx, err = f(ctx, w, r); err != nil {
			handleErr(compliance, err, r, w, acceptType)
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
			if len(srv.webApps) > 0 {
				http.Redirect(w, r, srv.webApps[0].endpoint, http.StatusMovedPermanently)
				return
			}
		}
	}

	op1, deviceId, p := shiftOptionalParamWithinSegment(r.URL, '=', '/')
	device, err := srv.findDevice(deviceId)
	if err != nil {
		handleErr(compliance, err, r, w, acceptType)
		return
	}
	switch op1 {
	case ".ver":
		w.Write([]byte(srv.Ver))
		return
	case ".well-known":
		srv.serveStaticRoute(w, r)
		return
	case "restconf":
		op2, p := shift(p, '/')
		r.URL = p
		switch op2 {
		case "data":
			srv.serve(compliance, ctx, device, w, r, endpointData, acceptType)
		case "streams":
			srv.serve(compliance, ctx, device, w, r, endpointStreams, acceptType)
		case "operations":
			srv.serve(compliance, ctx, device, w, r, endpointOperations, acceptType)
		case "ui":
			srv.serveStreamSource(compliance, r, w, device.UiSource(), r.URL.Path, acceptType)
		case "schema":
			// Hack - parse accept header to get proper content type
			accept := r.Header.Get("Accept")
			fc.Debug.Printf("accept %s", accept)
			if strings.Contains(accept, "/json") {
				srv.serveSchema(compliance, ctx, w, r, device.SchemaSource(), acceptType)
			} else {
				srv.serveStreamSource(compliance, r, w, device.SchemaSource(), r.URL.Path, acceptType)
			}
		default:
			handleErr(compliance, ErrBadAddress, r, w, acceptType)
		}
		return
	}
	if srv.handleWebApp(w, r, op1, p.Path, acceptType) {
		return
	}
	if srv.UnhandledRequestHandler != nil {
		srv.UnhandledRequestHandler(w, r)
		return
	}
}

const (
	endpointData = iota
	endpointOperations
	endpointStreams
	endpointSchema
)

func (srv *Server) serveSchema(compliance ComplianceOptions, ctx context.Context, w http.ResponseWriter, r *http.Request, ypath source.Opener, accept MimeType) {
	modName, p := shift(r.URL, '/')
	r.URL = p
	m, err := parser.LoadModule(ypath, modName)
	if err != nil {
		handleErr(compliance, err, r, w, accept)
		return
	}
	ylib, err := parser.LoadModule(ypath, "fc-yang")
	if err != nil {
		handleErr(compliance, err, r, w, accept)
		return
	}
	b := nodeutil.SchemaBrowser(ylib, m)
	hndlr := &browserHandler{browser: b}
	hndlr.ServeHTTP(compliance, ctx, w, r, endpointSchema)
}

func (srv *Server) serve(compliance ComplianceOptions, ctx context.Context, d device.Device, w http.ResponseWriter, r *http.Request, endpointId int, accept MimeType) {
	if hndlr, p := srv.shiftBrowserHandler(compliance, r, d, w, r.URL, accept); hndlr != nil {
		r.URL = p
		hndlr.ServeHTTP(compliance, ctx, w, r, endpointId)
	}
}

type webApp struct {
	endpoint string
	homeDir  string
	homePage string
}

func (srv *Server) RegisterWebApp(homeDir string, homePage string, endpoint string) {
	srv.webApps = append(srv.webApps, webApp{
		endpoint: endpoint,
		homeDir:  homeDir,
		homePage: homePage,
	})
}

// Serve web app according to SPA conventions where you serve static assets if
// they exist but if they don't assume, the URL is going to be interpretted
// in browser as route path.
func (srv *Server) handleWebApp(w http.ResponseWriter, r *http.Request, endpoint string, path string, accept MimeType) bool {
	for _, wap := range srv.webApps {

		if endpoint == wap.endpoint {

			// if someone type "/app/index.html" then direct them to right spot
			if strings.HasPrefix(path, wap.homePage) {
				// redirect to root path so URL is correct in browser
				http.Redirect(w, r, wap.endpoint, http.StatusMovedPermanently)
				return true
			}

			// redirect to root path so URL is correct in browser
			srv.serveWebApp(w, r, wap, path, accept)
			return true
		}
	}
	return false
}

func (srv *Server) serveWebApp(w http.ResponseWriter, r *http.Request, wap webApp, path string, accept MimeType) {
	compliance := Simplified
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
				handleErr(compliance, ferr, r, w, accept)
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
				handleErr(compliance, fc.NotFoundError, r, w, accept)
			} else {
				handleErr(compliance, ferr, r, w, accept)
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
		handleErr(compliance, err, r, w, accept)
	}
}

func (srv *Server) serveStreamSource(compliance ComplianceOptions, r *http.Request, w http.ResponseWriter, s source.Opener, path string, accept MimeType) {
	rdr, err := s(path, "")
	if err != nil {
		handleErr(compliance, err, r, w, accept)
		return
	} else if rdr == nil {
		handleErr(compliance, fc.NotFoundError, r, w, accept)
		return
	}
	ext := filepath.Ext(path)
	ctype := mime.TypeByExtension(ext)
	w.Header().Set("Content-Type", ctype)
	if _, err := io.Copy(w, rdr); err != nil {
		handleErr(compliance, err, r, w, accept)
	}
}

func (srv *Server) findDevice(deviceId string) (device.Device, error) {
	if deviceId == "" {
		return srv.main, nil
	}
	device, err := srv.devices.Device(deviceId)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("%w. device %s", fc.NotFoundError, deviceId)
	}
	return device, nil
}

func (srv *Server) shiftBrowserHandler(compliance ComplianceOptions, r *http.Request, d device.Device, w http.ResponseWriter, orig *url.URL, accept MimeType) (*browserHandler, *url.URL) {
	if module, p := shift(orig, ':'); module != "" {
		if browser, err := d.Browser(module); browser != nil {
			return &browserHandler{
				browser: browser,
			}, p
		} else if err != nil {
			handleErr(compliance, err, r, w, accept)
			return nil, orig
		}
	}

	handleErr(compliance, fmt.Errorf("%w. no module found in path", fc.NotFoundError), r, w, accept)
	return nil, orig
}

func (srv *Server) serveStaticRoute(w http.ResponseWriter, r *http.Request) bool {
	_, p := shift(r.URL, '/')
	op, _ := shift(p, '/')
	switch op {
	case "host-meta":
		// RESTCONF Sec. 3.1
		u, _ := url.Parse("/")
		fmt.Fprintf(w, `{ "subject": "%s", "links" : [ { "rel" : "restconf", "href" : "/restconf" } ] }`, r.URL.ResolveReference(u))
		return true
	}
	return false
}
