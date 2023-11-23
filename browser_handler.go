package restconf

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"context"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

type browserHandler struct {
	browser *node.Browser
}

var subscribeCount int

const EventTimeFormat = "2006-01-02T15:04:05-07:00"

type ProxyContextKey string

var RemoteIpAddressKey = ProxyContextKey("FC_REMOTE_IP")

type MimeType string

const (
	// TODO: Clarify this: RFC8572 uses application/yang.data+xml and RFC8040 uses application/yang-data+json
	YangDataJsonMimeType1 = MimeType("application/yang-data+json")
	YangDataJsonMimeType2 = MimeType("application/yang.data+json")

	YangDataXmlMimeType1 = MimeType("application/yang-data+xml")
	YangDataXmlMimeType2 = MimeType("application/yang.data+xml")

	PlainJsonMimeType = MimeType("application/json")

	TextStreamMimeType = MimeType("text/event-stream")
)

const SimplifiedComplianceParam = "simplified"

type ComplianceContextKeyType string

var ComplianceContextKey = ComplianceContextKeyType("RESTCONF_COMPLIANCE")

func (hndlr *browserHandler) ServeHTTP(compliance ComplianceOptions, ctx context.Context, w http.ResponseWriter, r *http.Request, endpointId int) {
	var err error
	var payload node.Node
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	if r.RemoteAddr != "" {
		host, _ := ipAddrSplitHostPort(r.RemoteAddr)
		ctx = context.WithValue(ctx, RemoteIpAddressKey, host)
	}
	sel := hndlr.browser.RootWithContext(ctx)
	var target *node.Selection
	defer sel.Release()
	if target, err = sel.FindUrl(r.URL); err == nil {
		acceptType := MimeType(r.Header.Get("Accept"))
		contentType := MimeType(r.Header.Get("Content-Type"))
		wireFmt := getWireFormatter(acceptType)
		hdr := w.Header()
		if target == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		defer target.Release()
		if handleErr(compliance, err, r, w) {
			return
		}
		isRpcOrAction := r.Method == "POST" && meta.IsAction(target.Meta())
		if !isRpcOrAction && endpointId == endpointOperations {
			http.Error(w, "{+restconf}/operations is only intended for rpcs", http.StatusBadRequest)
		} else if isRpcOrAction && !compliance.AllowRpcUnderData && endpointId == endpointData {
			isAction := target.Path.Len() > 2 // otherwise an action and ok
			if !isAction {
				http.Error(w, "rpcs are located at {+restconf}/operations not {+restconf}/data", http.StatusBadRequest)
				return
			}
		}
		switch r.Method {
		case "DELETE":
			// CRUD - Delete
			err = target.Delete()
		case "GET":
			if meta.IsNotification(target.Meta()) {
				hdr.Set("Content-Type", string(TextStreamMimeType)+"; charset=utf-8")
				hdr.Set("Cache-Control", "no-cache")
				hdr.Set("Connection", "keep-alive")
				hdr.Set("X-Accel-Buffering", "no")

				// TODO: Make CORS configurable
				hdr.Set("Access-Control-Allow-Origin", "*")

				// default is chunked and web browsers don't know to read after each flush
				hdr.Set("Transfer-Encoding", "identity")

				var sub node.NotifyCloser
				flusher, hasFlusher := w.(http.Flusher)
				if !hasFlusher {
					panic("invalid response writer")
				}
				flusher.Flush()

				subscribeCount++
				defer func() {
					subscribeCount--
				}()

				errOnSend := make(chan error, 20)
				origMod := meta.OriginalModule(target.Meta())
				sub, err = target.Notifications(func(n node.Notification) {
					defer func() {
						if r := recover(); r != nil {
							err := fmt.Errorf("recovered while attempting to send notification %s", r)
							errOnSend <- err
						}
					}()

					// write into a buffer so we write data all at once to handle concurrent messages and
					// ensure messages are not corrupted.  We could use a lock, but might cause deadlocks
					var buf bytes.Buffer

					// According to SSE Spec, each event needs following format:
					// data: {payload}\n\n
					fmt.Fprint(&buf, "data: ")
					if !compliance.DisableNotificationWrapper {
						etime := n.EventTime.Format(EventTimeFormat)
						wireFmt.writeNotificationStart(&buf, origMod, etime)
					}
					err := n.Event.InsertInto(nodeWtr(acceptType, compliance, &buf))
					if err != nil {
						errOnSend <- err
						return
					}
					if !compliance.DisableNotificationWrapper {
						wireFmt.writeNotificationEnd(&buf)
					}
					fmt.Fprint(&buf, "\n\n")
					_, err = w.Write(buf.Bytes())
					if err != nil {
						errOnSend <- fmt.Errorf("error writing notif. %s", err)
					}
					flusher.Flush()
					fc.Debug.Printf("sent %d bytes in notif", buf.Len())
				})
				if err != nil {
					fc.Err.Print(err)
					return
				}
				defer sub()
				select {
				case <-r.Context().Done():
					// normal client closing subscription
				case err = <-errOnSend:
					fc.Err.Print(err)
				}
				return
			} else {
				// CRUD - Read
				setContentType(compliance, w.Header())
				err = target.InsertInto(nodeWtr(acceptType, compliance, w))
			}
		case "PATCH":
			// CRUD - Upsert
			var input node.Node
			input, err = requestNode(r)
			if err != nil {
				handleErr(compliance, err, r, w)
				return
			}
			err = target.UpsertFrom(input)
		case "PUT":
			// CRUD - Remove and replace
			var input node.Node
			input, err = requestNode(r)
			if err != nil {
				handleErr(compliance, err, r, w)
				return
			}
			err = target.ReplaceFrom(input)
		case "POST":
			if meta.IsAction(target.Meta()) {
				// RPC
				a := target.Meta().(*meta.Rpc)
				var input node.Node
				if a.Input() != nil && r.ContentLength > 0 {
					if input, err = readInput(compliance, contentType, r, a); err != nil {
						handleErr(compliance, err, r, w)
						return
					}
				}
				outputSel, err := target.Action(input)
				if err != nil {
					handleErr(compliance, err, r, w)
					return
				}
				if outputSel != nil && a.Output() != nil {
					setContentType(compliance, w.Header())
					if err = sendActionOutput(acceptType, compliance, wireFmt, w, outputSel, a); err != nil {
						handleErr(compliance, err, r, w)
						return
					}
				} else {
					// Successfully processed POST but nothing to return
					w.WriteHeader(http.StatusNoContent)
				}
			} else {
				// CRUD - Insert
				payload = nodeutil.ReadJSONIO(r.Body)
				err = target.InsertFrom(payload)
			}
		case "OPTIONS":
			// NOP
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	}

	if err != nil {
		handleErr(compliance, err, r, w)
	}
}

func setContentType(compliance ComplianceOptions, h http.Header) {
	if compliance.QualifyNamespaceDisabled {
		h.Set("Content-Type", mime.TypeByExtension(".json"))
	} else {
		h.Set("Content-Type", string(YangDataJsonMimeType1))
	}
}

func sendActionOutput(acceptType MimeType, compliance ComplianceOptions, wireFormat wireFormat, out io.Writer, output *node.Selection, a *meta.Rpc) error {
	if !compliance.DisableActionWrapper {
		// IETF formated output
		// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.2
		mod := meta.OriginalModule(a)
		if _, err := wireFormat.writeRpcOutputStart(out, mod); err != nil {
			return err
		}
	}
	err := output.InsertInto(nodeWtr(acceptType, compliance, out))

	if !compliance.DisableActionWrapper {
		if _, err := wireFormat.writeRpcOutputEnd(out); err != nil {
			return err
		}
	}
	return err
}

func nodeWtr(mime MimeType, compliance ComplianceOptions, out io.Writer) node.Node {
	if mime.IsXml() {
		wtr := &nodeutil.XMLWtr{
			Out: out,
		}
		return wtr.Node()
	}
	wtr := &nodeutil.JSONWtr{
		Out:              out,
		QualifyNamespace: !compliance.QualifyNamespaceDisabled,
	}
	return wtr.Node()
}

func nodeRdr(mime MimeType, in io.Reader) (node.Node, error) {
	if mime.IsXml() {
		return nodeutil.ReadXMLBlock(in)
	}
	return nodeutil.ReadJSONIO(in), nil
}

func readInput(compliance ComplianceOptions, contentType MimeType, r *http.Request, a *meta.Rpc) (node.Node, error) {
	// not part of spec, custom feature to allow for form uploads
	if isMultiPartForm(r.Header) {
		return formNode(r)
	}
	n, err := nodeRdr(contentType, r.Body)
	if err != nil {
		return nil, err
	}
	if compliance.DisableActionWrapper {
		return n, nil
	}
	m := meta.OriginalModule(a)
	// IETF formated input
	// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.1
	n, err = findNodeOutsideSchema(m, "input", n)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, fmt.Errorf("missing in input wrapper %s", meta.SchemaPath(a))
	}
	return n, nil
}

func requestNode(r *http.Request) (node.Node, error) {
	// not part of spec, custom feature to allow for form uploads
	if isMultiPartForm(r.Header) {
		return formNode(r)
	}

	return nodeutil.ReadJSONIO(r.Body), nil
}

func (m MimeType) IsXml() bool {
	return strings.HasSuffix(string(m), "xml")
}

func (m MimeType) IsJson() bool {
	return strings.HasSuffix(string(m), "json")
}

func (m MimeType) IsRfc() bool {
	return m == YangDataJsonMimeType1 || m == YangDataJsonMimeType2 || m == YangDataXmlMimeType1 || m == YangDataXmlMimeType2
}

func findNodeOutsideSchema(m *meta.Module, container string, n node.Node) (node.Node, error) {
	// create a new module on the fly with just a single container and immediately
	// select that container.
	bldr := &meta.Builder{}
	copy := bldr.Module(m.Ident(), m.FeatureSet())
	bldr.Namespace(copy, m.Namespace())
	bldr.Container(copy, container)
	b := node.NewBrowser(copy, n)
	sel, err := b.Root().Find(container)
	if sel == nil || err != nil {
		return nil, err
	}
	return sel.Node, err
}
