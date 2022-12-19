package restconf

import (
	"container/list"
	"fmt"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
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
	Endpoint     string
	LocalAddress string
	RetryRateMs  int
}

func NewCallHome(registrarProto device.ProtocolHandler) *CallHome {
	return &CallHome{
		registrarProto: registrarProto,
		listeners:      list.New(),
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
	if callh.options == options {
		return nil
	}
	callh.options = options
	callh.Registered = false
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
	regUrl := callh.options.Address + callh.options.Endpoint
	registrar, err := callh.registrarProto(regUrl)
	if err != nil {
		fc.Err.Printf("failed to build device with address %s. %s", regUrl, err)
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

func (callh *CallHome) register(registrar device.Device) error {
	modname := "fc-call-home-server"
	reg, err := registrar.Browser(modname)
	if err != nil {
		return err
	}
	if reg == nil {
		return fmt.Errorf("%s module not found on remote target", modname)
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
