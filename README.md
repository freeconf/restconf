# ![FreeCONF](https://s3.amazonaws.com/freeconf-static/freeconf-no-wrench.svg)

For more information about this project, [see wiki](https://github.com/freeconf/restconf/wiki).

# RESTCONF client and server library

This is a library for adding support for configuration, metrics, operations and events to any service written in the Go programming language.  It supports [IETF RESTCONF RFC8040](https://tools.ietf.org/html/rfc8040) protocol so it can interoperate with other libraries and systems. Even if you do not currently use these standards, this library gives you a powerful management system based on decades of engineering.

# FreeCONF Mission

FreeCONF plays an important role in the greater mission to browse, inspect and program __every piece__ of running software in your entire IT infrastructure! FreeCONF uses IETF standards to support configuration, metrics, operations and events to any service written in the Go programming language.

# Requirements

Requires Go version 1.9 or greater.

# Getting the source

```bash
go get -u github.com/freeconf/yang
```

# What can I do with this library?

1. Never write a configuration file parser again
2. Generate accurate management documentation
3. Support live configuration changes such as log level
4. Decouple your code from any management tool (prometheus, slack, influx)
5. Support RBAC (Roll Based Access and Control) for mangagement operations without any code change
6. Build web-based management interfaces without needing any extra service
7. Use call-home to build, automatic inventory database
8.  Wrap messy integration APIs to other systems
9.  Track exact changes to APIs for semantic versioning

Once you get started, there are a surprising number of possibilities including non-manamgent APIs, file parsers, DB schema generation, ...

# Example

### Step 1 - Create a Go project modeling a car
```bash
mkdir car
cd car
go mod init car
go get -u github.com/freeconf/restconf
```

### Step 2 - Get root model files

There are some model files needed to start a web server and other basic things.

```bash
go run github.com/freeconf/yang/cmd/fc-yang get
```

you should now see bunch of *.yang files in the current directory.  They were actually extracted from the source, not downloaded.

### Step 3 - Write your own model file

Use [YANG](https://tools.ietf.org/html/rfc6020) to model your management API by creating the following file called `car.yang` with the following contents.

```YANG
module car {
	description "Car goes beep beep";

	revision 0;

	leaf speed {
		description "How fast the car goes";
	    type int32 {
		    range "0..120";
	    }
		units milesPerSecond;
	}

	leaf miles {
		description "How many miles has car moved";
	    type decimal64;
	    config false;
	}

	rpc reset {
		description "Reset the odometer";
	}
}
```

### Step 4 - Write a program, its management API and a main entry point

Create a go source file called `main.go` with the following contents.

```go
package main

import (
	"strings"
	"time"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
)

// write your code to capture your domain however you want
type Car struct {
	Speed int
	Miles float64
}

func (c *Car) Start() {
	for {
		<-time.After(time.Second)
		c.Miles += float64(c.Speed)
	}
}

// write mangement api to bridge from YANG to code
func manage(car *Car) node.Node {
	return &nodeutil.Extend{

		// use reflect when possible, here we're using to get/set speed AND
		// to read miles metrics.
		Base: nodeutil.ReflectChild(car),

		// handle action request
		OnAction: func(parent node.Node, req node.ActionRequest) (node.Node, error) {
			switch req.Meta.Ident() {
			case "reset":
				car.Miles = 0
			}
			return nil, nil
		},
	}
}

// Connect everything together into a server to start up
func main() {

	// Your app
	car := &Car{}

	// Device can hold multiple modules, here we are only adding one
	d := device.New(source.Path("."))
	if err := d.Add("car", manage(car)); err != nil {
		panic(err)
	}

	// Select wire-protocol RESTCONF to serve the device.
	restconf.NewServer(d)

	// apply start-up config normally stored in a config file on disk
	config := `{
		"fc-restconf":{"web":{"port":":8080"}},
        "car":{"speed":10}
	}`
	if err := d.ApplyStartupConfig(strings.NewReader(config)); err != nil {
		panic(err)
	}

	// start your app
	car.Start()
}
```

### Step 5. Now run your program

Start your application

```bash
go run . &
```

You will see a warning about HTTP2, but you can ignore that.  Once you install a web certificate, that will go away.

#### Get Configuration
`curl http://localhost:8080/restconf/data/car:`

```json
{"speed":10,"miles":450}
```

#### Change Configuration
`curl -XPUT http://localhost:8080/restconf/data/car: -d '{"speed":99}'`

#### Reset odometer
`curl -XPOST http://localhost:8080/restconf/data/car:reset`

## Resources
* [Wiki](https://github.com/freeconf/restconf/wiki)
* [Discussions](https://github.com/freeconf/restconf/discussions)
* [Issues](https://github.com/freeconf/restconf/issues)

## RFCs

If you don't see an RFC here, open a discussion to see if there is interest or existing implementations.

* [RFC 8040](https://datatracker.ietf.org/doc/html/rfc8040) - RESTCONF (sans XML)
* [RFC 8525](https://datatracker.ietf.org/doc/html/rfc8525) - YANG Library
* [RFCs from underlying yang library](https://github.com/freeconf/yang#rfcs)