package restconf

import (
	"testing"

	"github.com/freeconf/yang/fc"
)

func TestFindDeviceIdInUrl(t *testing.T) {
	dev := FindDeviceIdInUrl("http://server:port/restconf=abc/")
	fc.AssertEqual(t, "abc", dev)
	dev = FindDeviceIdInUrl("http://server:port/restconf/")
	fc.AssertEqual(t, "", dev)
}
