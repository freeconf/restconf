package stock

import (
	"context"
	"crypto/tls"
	"io"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
)

type HttpServerOptions struct {
	Addr                     string
	Port                     string
	ReadTimeout              int
	WriteTimeout             int
	Tls                      *Tls
	Iface                    string
	CallbackAddress          string
	NotifyKeepaliveTimeoutMs int
}

type HttpServer struct {
	options HttpServerOptions
	Server  *http.Server
	handler http.Handler
	Metrics WebMetrics
}

func (service *HttpServer) Options() HttpServerOptions {
	return service.options
}

func (service *HttpServer) ApplyOptions(options HttpServerOptions) {
	if options == service.options {
		return
	}
	service.options = options
	service.Server = &http.Server{
		Addr:           options.Port,
		Handler:        service.handler,
		ReadTimeout:    time.Duration(options.ReadTimeout) * time.Millisecond,
		WriteTimeout:   time.Duration(options.WriteTimeout) * time.Millisecond,
		MaxHeaderBytes: 1 << 20,
		ConnState:      service.connectionUpdate,
	}
	chkStartErr := func(err error) {
		if err != nil && err != http.ErrServerClosed {
			fc.Err.Fatal(err)
		}
	}
	if options.Tls != nil {
		service.Server.TLSConfig = &options.Tls.Config
		go func() {
			// Using "tcp" listener allowed for greater config flexibility for cert
			// data but disabled HTTP/2
			chkStartErr(service.Server.ListenAndServeTLS(options.Tls.CertFile, options.Tls.KeyFile))
		}()
	} else {
		// This really is an error, spec says RESTCONF w/o HTTPS should not be allowed.
		fc.Err.Printf("Without TLS configuration, HTTP2 cannot be enabled and notifications will be severly limited in web browsers")
		go func() {
			chkStartErr(service.Server.ListenAndServe())
		}()
	}
}

type WebMetrics struct {
	New      int64
	Active   int64
	Idle     int64
	Hijacked int64
	Closed   int64
}

func (service *HttpServer) connectionUpdate(conn net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		service.Metrics.New++
	case http.StateActive:
		service.Metrics.Active++
	case http.StateIdle:
		service.Metrics.Idle++
	case http.StateHijacked:
		service.Metrics.Hijacked++
	case http.StateClosed:
		service.Metrics.Closed++
	}
}

func (service *HttpServer) Stop() {
	service.Server.Shutdown(context.Background())
}

func NewHttpServer(handler http.Handler) *HttpServer {
	return &HttpServer{
		handler: handler,
	}
}

func (service *HttpServer) GetHttpClient() *http.Client {
	var client *http.Client
	if service.options.Tls != nil {
		tlsConfig := &tls.Config{
			Certificates: service.options.Tls.Config.Certificates,
			RootCAs:      service.options.Tls.Config.RootCAs,
		}
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: transport}
	} else {
		client = http.DefaultClient
	}
	return client
}

type StreamSourceWebHandler struct {
	Source source.Opener
}

func (service StreamSourceWebHandler) ServeHTTP(wtr http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "" {
		path = "index.html"
	}
	if rdr, err := service.Source(path, ""); err != nil {
		http.Error(wtr, err.Error(), 404)
	} else {
		if closer, ok := rdr.(io.Closer); ok {
			defer closer.Close()
		}
		ext := filepath.Ext(path)
		ctype := mime.TypeByExtension(ext)
		wtr.Header().Set("Content-Type", ctype)
		if _, err = io.Copy(wtr, rdr); err != nil {
			http.Error(wtr, err.Error(), http.StatusInternalServerError)
		}
		// Eventually support this but need file seeker to do that.
		// http.ServeContent(wtr, req, path, time.Now(), &ReaderPeeker{rdr})
	}
}

func WebServerNode(service *HttpServer) node.Node {
	options := service.Options()
	return &nodeutil.Extend{
		Base: nodeutil.ReflectChild(&options),
		OnChild: func(p node.Node, r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "tls":
				if r.New {
					options.Tls = &Tls{}
				}
				if options.Tls != nil {
					return TlsNode(options.Tls), nil
				}
			case "metrics":
				return nodeutil.ReflectChild(&service.Metrics), nil
			}
			return nil, nil
		},
		OnEndEdit: func(p node.Node, r node.NodeRequest) error {
			if err := p.EndEdit(r); err != nil {
				return err
			}
			service.ApplyOptions(options)
			return nil
		},
	}
}
