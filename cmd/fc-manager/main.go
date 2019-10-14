package main

import (
	"flag"
	"log"
	"os"

	"github.com/freeconf/manage/secure"
	"github.com/freeconf/yang/source"

	"github.com/freeconf/manage/device"
	"github.com/freeconf/manage/restconf"
	"github.com/freeconf/yang/c2"
)

// Initialize and start our RESTCONF proxy service.
//
// To run:
//    cd ./src/vendor/github.com/freeconf/yang/examples/proxy
//    go run ./main.go
//
// Then open web browser to
//   http://localhost:8080/restconf/ui/index.html
//

var startup = flag.String("startup", "startup.json", "startup configuration file.")
var verbose = flag.Bool("verbose", false, "verbose")

func main() {
	flag.Parse()
	c2.DebugLog(*verbose)

	// where UI files are stored
	uiPath := source.Dir("../web")

	// where all yang files are stored just for the server
	// models for devices that register are pulled automatically
	ypathEnv := os.Getenv("YANGPATH")
	if ypathEnv == "" {
		log.Fatal("YANGPATH environment variable not set")
	}
	ypath := source.Path(ypathEnv)

	// Even though this is a server component, we still organize things thru a device
	// because this server will appear like a "Device" to application management systems
	// "northbound"" representing all the devices that are "southbound".
	d := device.NewWithUi(ypath, uiPath)

	a := secure.NewRbac()
	d.Add("secure", secure.Manage(a))

	// Add RESTCONF service
	mgmt := restconf.NewServer(d)
	mgmt.Auth = a

	// Exposing your device manager means you can represent other devices
	dm := device.NewMap()
	mgmt.ServeDevices(dm)
	m := device.NewMap()
	chkErr(d.Add("map", device.MapNode(m)))

	// bootstrap config for all local modules
	chkErr(d.ApplyStartupConfigFile(*startup))

	// Wait for cntrl-c...
	select {}
}

func chkErr(err error) {
	if err != nil {
		panic(err)
	}
}
