package restconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"strings"

	"github.com/freeconf/yang/fc"
)

// SplitAddress takes a complete address and breaks it into pieces according
// to RESTCONF standards so you can use each piece in appropriate API call
// Example:
//
//	http://server[:port]/restconf[=device]/module:path/here
func SplitAddress(fullurl string) (address string, module string, path string, err error) {
	eoSlashSlash := strings.Index(fullurl, "//") + 2
	if eoSlashSlash < 2 {
		err = ErrBadAddress
		return
	}
	eoSlash := eoSlashSlash + strings.IndexRune(fullurl[eoSlashSlash:], '/') + 1
	if eoSlash <= eoSlashSlash {
		err = ErrBadAddress
		return
	}
	colon := eoSlash + strings.IndexRune(fullurl[eoSlash:], ':')
	if colon <= eoSlash {
		err = ErrBadAddress
		return
	}
	moduleBegin := strings.LastIndex(fullurl[:colon], "/")
	address = fullurl[:moduleBegin+1]
	module = fullurl[moduleBegin+1 : colon]
	path = fullurl[colon+1:]
	return
}

func SplitUri(uri string) (module string, path string, err error) {
	colon := strings.IndexRune(uri, ':')
	if colon < 0 {
		err = ErrBadAddress
		return
	}
	module = uri[:colon]
	if slash := strings.LastIndex(module, "/"); slash >= 0 {
		module = module[slash+1:]
	}
	path = uri[colon+1:]
	return
}

// FindDeviceIdInUrl picks out device id in URL
func FindDeviceIdInUrl(addr string) string {
	segs := strings.SplitAfter(addr, "/restconf=")
	if len(segs) == 2 {
		post := segs[1]
		return post[:len(post)-1]
	}
	return ""
}

// only call this when you know that no content has been sent to client
// otherwise go will emit error that you're trying to change header when
// it's too late.  i think harmless, but still not what you intended and
// actuall error is eatten.
func handleErr(compliance ComplianceOptions, err error, r *http.Request, w http.ResponseWriter) bool {
	if err == nil {
		return false
	}
	fc.Debug.Printf("web request error [%s] %s %s", r.Method, r.URL, err.Error())
	msg := err.Error()
	code := fc.HttpStatusCode(err)
	if !compliance.SimpleErrorResponse {
		errResp := errResponse{
			Type:    "protocol",
			Tag:     decodeErrorTag(code, err),
			Path:    decodeErrorPath(r.RequestURI),
			Message: msg,
		}
		var buff bytes.Buffer
		fmt.Fprintf(&buff, `{"ietf-restconf:errors":{"error":[`)
		json.NewEncoder(&buff).Encode(&errResp)
		fmt.Fprintf(&buff, `]}}`)
		msg = buff.String()
	}
	http.Error(w, msg, code)
	return true
}

// https://datatracker.ietf.org/doc/html/rfc8040#section-7
func decodeErrorTag(code int, _err error) string {
	// This is bare minimum to return formatted error message response.
	// but also all that can be done until more error types are defined
	// beyond the few in github.com/freeconf/yang/fc/err.go or a more
	// flexible error handling is implemented
	switch code {
	case 409:
		return "in-use"
	case 400:
		return "invalid-value"
	case 401:
		return "access-denied"
	}
	return "operation-failed"
}

func decodeErrorPath(fullPath string) string {
	module, path, err := SplitUri(fullPath)
	if err != nil {
		fc.Debug.Printf("unexpected path '%s', %s", fullPath, err)
		return fullPath
	}
	return fmt.Sprint(module, ":", path)
}

type errResponse struct {
	Type    string `json:"error-type"`
	Tag     string `json:"error-tag"`
	Path    string `json:"error-path"`
	Message string `json:"error-message"`
}

func ipAddrSplitHostPort(addr string) (host string, port string) {
	bracket := strings.IndexRune(addr, ']')
	dblColon := strings.Index(addr, "::")
	isIpv6 := (bracket >= 0 || dblColon >= 0)
	if isIpv6 {
		if bracket > 0 {
			host = addr[:bracket+1]
			if len(addr) > bracket+2 {
				port = addr[bracket+2:]
			}
		} else {
			host = addr
		}
	} else {
		colon := strings.IndexRune(addr, ':')
		if colon < 0 {
			host = addr
		} else {
			host = addr[:colon]
			port = addr[colon+1:]
		}
	}
	return
}

func appendUrlSegment(a string, b string) string {
	if a == "" || b == "" {
		return a + b
	}
	slashA := a[len(a)-1] == '/'
	slashB := b[0] == '/'
	if slashA != slashB {
		return a + b
	}
	if slashA && slashB {
		return a + b[1:]
	}
	return a + "/" + b
}

func shift(orig *url.URL, delim rune) (string, *url.URL) {
	if orig.Path == "" {
		return "", orig
	}
	copy := *orig
	var segment string
	segment, copy.Path = shiftInString(copy.Path, delim)
	_, copy.RawPath = shiftInString(copy.RawPath, delim)
	return segment, &copy
}

func shiftInString(orig string, delim rune) (string, string) {
	termPos := strings.IndexRune(orig, delim)

	// deisgn decision : ignore when path starts with the delim
	if termPos == 0 {
		orig = orig[1:]
		termPos = strings.IndexRune(orig, delim)
	}

	var shifted string
	var segment string
	if termPos < 0 {
		segment = orig
		// shifted = empty
	} else {
		segment = orig[:termPos]
		shifted = orig[termPos+1:]
	}
	return segment, shifted
}

func shiftOptionalParamWithinSegment(orig *url.URL, optionalDelim rune, segDelim rune) (string, string, *url.URL) {
	copy := *orig
	var segment, optional string
	// trickery here - mutating a copy of the URL
	segment, optional, copy.Path = shiftOptionalParamWithinSegmentInString(copy.Path, optionalDelim, segDelim)

	// NOTE: the segment and optional param are returned unescaped presumably because caller
	// would want that.  If not, keep these results and not the ones from above
	_, _, copy.RawPath = shiftOptionalParamWithinSegmentInString(copy.RawPath, optionalDelim, segDelim)

	return segment, optional, &copy
}

// this will not work of unescaped paths that contain optionalDelim or segDelim in the part of the
// url it's trying to shift.
func shiftOptionalParamWithinSegmentInString(orig string, optionalDelim rune, segDelim rune) (string, string, string) {
	termPos := strings.IndexRune(orig, segDelim)

	// design decision : ignore when path starts with the delim
	if termPos == 0 {
		orig = orig[1:]
		termPos = strings.IndexRune(orig, segDelim)
	}

	// find the next segment first...
	var shifted string
	var segment string
	if termPos < 0 {
		segment = orig
		// shifted = empty
	} else {
		segment = orig[:termPos]
		shifted = orig[termPos+1:]
	}

	// ...now look for optional param in the found segment
	optPos := strings.IndexRune(segment, optionalDelim)
	if optPos < 0 {
		return segment, "", shifted
	}
	var optional string
	if len(segment) > optPos+1 {
		optional = segment[optPos+1:]
	}
	segment = segment[:optPos]

	return segment, optional, shifted
}
