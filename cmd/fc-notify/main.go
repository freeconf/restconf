package main

import (
	"log"
	"os"

	"context"

	"github.com/freeconf/restconf"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
)

// Subscribes to a notification and exits on first message
//
// this can be expanded to repeat indefinitely as an option
// or supply an alternate value for 'origin' should the default
// not be valid for some reason
//
//  http://server:port/restconf/streams/module:path?fc-device=car-advanced
//  http://server:port/restconf=device/streams/module:path
//
func main() {
	fc.DebugLog(true)
	if len(os.Args) != 2 {
		usage()
	}
	address, module, path, err := restconf.SplitAddress(os.Args[1])
	fc.Info.Printf("%s %s %s %v", address, module, path, err)
	if err != nil {
		panic(err)
	}

	ypathEnv := os.Getenv("YANGPATH")
	if ypathEnv == "" {
		log.Fatal("YANGPATH environment variable not set")
	}
	ypath := source.Path(ypathEnv)

	d, err := restconf.ProtocolHandler(ypath)(address)
	if err != nil {
		panic(err)
	}
	defer d.Close()
	b, err := d.Browser(module)
	if err != nil {
		panic(err)
	}
	wait := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unsubscribe, err := b.RootWithContext(ctx).Find(path).Notifications(func(payload node.Notification) {
		wtr := &nodeutil.JSONWtr{Out: os.Stdout}
		if err = payload.Event.InsertInto(wtr.Node()).LastErr; err != nil {
			log.Fatal(err)
		}
		wait <- true
	})
	defer unsubscribe()
	if err != nil {
		log.Fatal(err)
	}
	<-wait
}

func usage() {
	log.Fatalf(`usage : %s http://server:port/restconf/module:path/some=x/where`, os.Args[0])
}
