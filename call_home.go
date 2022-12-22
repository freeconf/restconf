package restconf

import (
	"container/list"
	"fmt"
	"os"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

// Implements RFC Draft
//   https://www.rfc-editor.org/rfc/rfc8071.html
//
type CallHome struct {
	options        CallHomeOptions
	registrarProto device.ProtocolHandler
	Registered     bool
	registrar      device.Device // handle to the remote controller
	LastErr        string
	listeners      *list.List
}

type CallHomeOptions struct {
	DeviceId     string
	Address      string
	LocalAddress string
	RetryRateMs  int
}

func DefaultCallHomeOptions() CallHomeOptions {
	return CallHomeOptions{
		DeviceId: os.Getenv("DEVICE_ID"),
		Address:  os.Getenv("CALLHOME_ADDR"),
	}
}

func NewCallHome(registrarProto device.ProtocolHandler) *CallHome {
	return &CallHome{
		registrarProto: registrarProto,
		listeners:      list.New(),
		options:        DefaultCallHomeOptions(),
	}
}

type RegisterUpdate int

const (
	Register RegisterUpdate = iota
	Unregister
)

type RegisterListener func(d device.Device, update RegisterUpdate)

func (callh *CallHome) OnRegister(l RegisterListener) nodeutil.Subscription {
	if callh.Registered {
		l(callh.registrar, Register)
	}
	return nodeutil.NewSubscription(callh.listeners, callh.listeners.PushBack(l))
}

func (callh *CallHome) Options() CallHomeOptions {
	return callh.options
}

func (callh *CallHome) ApplyOptions(options CallHomeOptions) error {
	if nonfatal := callh.unregister(); nonfatal != nil {
		fc.Err.Printf("could not unregister. %s", nonfatal)
	}
	callh.options = options
	callh.Registered = false
	if callh.options.Address == "" {
		fc.Debug.Print("no call home address configured")
		return nil
	}
	fc.Debug.Print("connecting to ", callh.options.Address)
	callh.Register()
	return nil
}

func (callh *CallHome) updateListeners(registrar device.Device, update RegisterUpdate) {
	callh.registrar = registrar
	p := callh.listeners.Front()
	for p != nil {
		p.Value.(RegisterListener)(callh.registrar, update)
		p = p.Next()
	}
}

func (callh *CallHome) Register() {
retry:
	registrar, err := callh.registrarProto(callh.options.Address)
	if err != nil {
		fc.Err.Printf("failed to build device with address %s. %s", callh.options.Address, err)
	} else {
		if err = callh.register(registrar); err != nil {
			fc.Err.Printf("failed to register %s", err)
		} else {
			return
		}
	}
	if callh.options.RetryRateMs == 0 {
		panic("failed to register and no retry rate configured")
	}
	<-time.After(time.Duration(callh.options.RetryRateMs) * time.Millisecond)
	goto retry
}

func (callh *CallHome) serverApi(registrar device.Device) (*node.Browser, error) {
	modname := "fc-call-home-server"
	reg, err := registrar.Browser(modname)
	if err != nil {
		return nil, err
	}
	if reg == nil {
		return nil, fmt.Errorf("%s module not found on remote target", modname)
	}
	return reg, nil
}

func (callh *CallHome) unregister() error {
	if !callh.Registered {
		return nil
	}
	registrar, err := callh.registrarProto(callh.options.Address)
	if err != nil {
		return err
	}
	defer func() {
		callh.updateListeners(registrar, Unregister)
		callh.Registered = false
	}()
	reg, err := callh.serverApi(registrar)
	if err != nil {
		return err
	}
	if err = reg.Root().Find("register").Action(nil).LastErr; err != nil {
		return err
	}
	return nil
}

func (callh *CallHome) register(registrar device.Device) error {
	reg, err := callh.serverApi(registrar)
	if err != nil {
		return err
	}
	r := map[string]interface{}{
		"deviceId": callh.options.DeviceId,
		"address":  callh.options.LocalAddress,
	}
	err = reg.Root().Find("register").Action(nodeutil.ReflectChild(r)).LastErr
	if err == nil {
		callh.updateListeners(registrar, Register)
		callh.Registered = true
	}
	return err
}
