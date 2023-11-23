package restconf

import (
	"fmt"
	"io"

	"github.com/freeconf/yang/meta"
)

func getWireFormatter(accept MimeType) wireFormat {
	if accept.IsXml() {
		return xmlWireFormat(0)
	}
	return jsonWireFormat(0)
}

type wireFormat interface {
	writeNotificationStart(w io.Writer, module *meta.Module, etime string) (int, error)
	writeNotificationEnd(w io.Writer) (int, error)
	writeRpcOutputStart(w io.Writer, module *meta.Module) (int, error)
	writeRpcOutputEnd(w io.Writer) (int, error)
}

type jsonWireFormat int

func (jsonWireFormat) writeNotificationStart(w io.Writer, module *meta.Module, etime string) (int, error) {
	return fmt.Fprintf(w, `{"ietf-restconf:notification":{"eventTime":"%s","event":`, etime)
}

func (jsonWireFormat) writeNotificationEnd(w io.Writer) (int, error) {
	return fmt.Fprint(w, "}}")
}

func (jsonWireFormat) writeRpcOutputStart(w io.Writer, module *meta.Module) (int, error) {
	return fmt.Fprintf(w, `{"%s:output":`, module.Ident())
}

func (jsonWireFormat) writeRpcOutputEnd(w io.Writer) (int, error) {
	return fmt.Fprint(w, "}")
}

type xmlWireFormat int

func (xmlWireFormat) writeNotificationStart(w io.Writer, module *meta.Module, etime string) (int, error) {
	return fmt.Fprintf(w, `<notification xmlns="urn:ietf:params:xml:ns:netconf:notif\
	ication:1.0"><eventTime>%s</eventTime><event xmlns="%s">`, etime, module.Namespace())
}

func (xmlWireFormat) writeNotificationEnd(w io.Writer) (int, error) {
	return fmt.Fprint(w, "</event></notification>")
}

func (xmlWireFormat) writeRpcOutputStart(w io.Writer, module *meta.Module) (int, error) {
	return 0, nil
}

func (xmlWireFormat) writeRpcOutputEnd(w io.Writer) (int, error) {
	return 0, nil
}
