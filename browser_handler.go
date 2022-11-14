package restconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"

	"context"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

const (
	allowOperationsAny = iota
	allowOperationsOnly
	allowOperationsNone
)

type browserHandler struct {
	browser             *node.Browser
	allowOperationsFlag int
}

var subscribeCount int

const eventTimeFormat = "2006-01-02T15:04:05-07:00"

func (self *browserHandler) ServeHTTP(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var err error
	var payload node.Node
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	if r.RemoteAddr != "" {
		host, _ := ipAddrSplitHostPort(r.RemoteAddr)
		ctx = context.WithValue(ctx, device.RemoteIpAddressKey, host)
	}
	sel := self.browser.RootWithContext(ctx)
	if sel = sel.FindUrl(r.URL); sel.LastErr == nil {
		hdr := w.Header()
		if sel.IsNil() {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if handleErr(err, r, w) {
			return
		}
		if self.allowOperationsFlag != allowOperationsAny {
			isOperation := r.Method == "POST" && meta.IsAction(sel.Meta()) && sel.Path.Len() == 2
			if self.allowOperationsFlag == allowOperationsOnly && !isOperation {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			if self.allowOperationsFlag == allowOperationsNone && isOperation {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}
		switch r.Method {
		case "DELETE":
			// CRUD - Delete
			err = sel.Delete()
		case "GET":
			// compliance note : decided to support notifictions on get by devilering
			// first event, then closing connection.  Spec calls for SSE
			if meta.IsNotification(sel.Meta()) {
				hdr.Set("Content-Type", "text/event-stream")
				hdr.Set("Cache-Control", "no-cache")
				hdr.Set("Connection", "keep-alive")
				hdr.Set("X-Accel-Buffering", "no")
				hdr.Set("Access-Control-Allow-Origin", "*")
				// default is chunked and web browsers don't know to read after each
				// flush
				hdr.Set("Transfer-Encoding", "identity")

				var sub node.NotifyCloser
				flusher, hasFlusher := w.(http.Flusher)
				if !hasFlusher {
					panic("invalid response writer")
				}
				subscribeCount++
				defer func() {
					subscribeCount--
				}()

				errOnSend := make(chan error, 20)
				sub, err = sel.Notifications(func(n node.Notification) {
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
					if !Compliance.DisableNotificationWrapper {
						etime := n.EventTime.Format(eventTimeFormat)
						fmt.Fprintf(&buf, `{"ietf-restconf:notification":{"eventTime":"%s","event":`, etime)
					}
					jout := &nodeutil.JSONWtr{Out: &buf}

					err := n.Event.InsertInto(jout.Node()).LastErr
					if err != nil {
						errOnSend <- err
						return
					}
					if !Compliance.DisableNotificationWrapper {
						fmt.Fprint(&buf, "}}")
					}
					fmt.Fprint(&buf, "\n\n")
					w.Write(buf.Bytes())
					flusher.Flush()
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
				hdr.Set("Content-Type", mime.TypeByExtension(".json"))
				jout := &nodeutil.JSONWtr{Out: w}
				err = sel.InsertInto(jout.Node()).LastErr
			}
		case "PUT":
			// CRUD - Update
			var input node.Node
			input, err = requestNode(r)
			if err != nil {
				handleErr(err, r, w)
				return
			}
			err = sel.UpsertFrom(input).LastErr
		case "POST":
			if meta.IsAction(sel.Meta()) {
				// RPC
				a := sel.Meta().(*meta.Rpc)
				var input node.Node
				if a.Input() != nil {
					if input, err = readInput(r, a); err != nil {
						handleErr(err, w)
						return
					}
				}
				if outputSel := sel.Action(input); !outputSel.IsNil() && a.Output() != nil {
					w.Header().Set("Content-Type", mime.TypeByExtension(".json"))
					if err = sendOutput(w, outputSel, a); err != nil {
						handleErr(err, w)
						return
					}
				} else {
					err = outputSel.LastErr
				}
			} else {
				// CRUD - Insert
				payload = nodeutil.ReadJSONIO(r.Body)
				err = sel.InsertFrom(payload).LastErr
			}
		case "OPTIONS":
			// NOP
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	} else {
		err = sel.LastErr
	}

	if err != nil {
		handleErr(err, r, w)
	}
}

func sendOutput(out io.Writer, output node.Selection, a *meta.Rpc) error {
	if !Compliance.DisableActionWrapper {
		// IETF formated output
		// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.2
		mod := meta.OriginalModule(a).Ident()
		if _, err := fmt.Fprintf(out, `{"%s:output":`, mod); err != nil {
			return err
		}
	}
	jout := &nodeutil.JSONWtr{Out: out}
	err := output.InsertInto(jout.Node()).LastErr

	if !Compliance.DisableActionWrapper {
		if _, err := fmt.Fprintf(out, "}"); err != nil {
			return err
		}
	}
	return err
}

func readInput(r *http.Request, a *meta.Rpc) (node.Node, error) {
	// not part of spec, custom feature to allow for form uploads
	if isMultiPartForm(r.Header) {
		return formNode(r)
	} else if Compliance.DisableActionWrapper {
		return nodeutil.ReadJSONIO(r.Body), nil
	}

	// IETF formated input
	// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.1
	var vals map[string]interface{}
	d := json.NewDecoder(r.Body)
	err := d.Decode(&vals)
	if err != nil {
		return nil, err
	}
	key := meta.OriginalModule(a).Ident() + ":input"
	payload, found := vals[key].(map[string]interface{})
	if !found {
		return nil, fmt.Errorf("'%s' missing in input wrapper", key)
	}
	return nodeutil.ReadJSONValues(payload), nil
}

func requestNode(r *http.Request) (node.Node, error) {
	// not part of spec, custom feature to allow for form uploads
	if isMultiPartForm(r.Header) {
		return formNode(r)
	}

	return nodeutil.ReadJSONIO(r.Body), nil
}
