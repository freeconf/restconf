package restconf

import (
	"testing"

	"github.com/freeconf/yang/fc"
)

func Test_findDeviceIdInUrl(t *testing.T) {
	dev := findDeviceIdInUrl("http://server:port/restconf=abc/")
	fc.AssertEqual(t, "abc", dev)
	dev = findDeviceIdInUrl("http://server:port/restconf/")
	fc.AssertEqual(t, "", dev)
}
