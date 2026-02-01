package joybus

import (
	"encoding/binary"
	"sort"

	"github.com/rasky/g64drive/ultrasave/logger"
)

// Abstact Joybus interface
type Controller interface {
	Execute(cmd Command, tx, rx []byte) error
	Unfreeze(err error) bool
}

// Joybus commands are 8 bits.
type Command uint8

// Joybus common commands.
// Commands specific to each device are defined in their respective packages.
const (
	cmdInfo  = Command(0x00)
	cmdReset = Command(0xff)
)

type DeviceID uint16
type Status uint8

type Device interface {
	// Assume all joybus devices support Info command
	Info() (DeviceID, Status, error)
	// TOVERIFY: does all joybus devices support this ?
	Reset() (DeviceID, Status, error)
}

// XXX: not sure it helps much
type DeviceImpl struct {
	Controller
}

// Ensure *DeviceImpl implements Device interface at compile time.
var _ Device = (*DeviceImpl)(nil)

func (d *DeviceImpl) Info() (DeviceID, Status, error) {
	return Info(d, cmdInfo)
}

func (d *DeviceImpl) Reset() (DeviceID, Status, error) {
	return Info(d, cmdReset)
}

func Info(joybus Controller, cmd Command) (DeviceID, Status, error) {
	var info [3]byte

	if err := joybus.Execute(cmd, nil, info[:]); err != nil {
		return 0, 0, err
	}

	id := DeviceID(binary.BigEndian.Uint16(info[:2]))
	status := Status(info[2])

	return id, status, nil
}

type Factory struct {
	Name    string
	Factory func(DeviceID, Controller) Device
}

type Factories map[DeviceID]Factory

type OrderedProbingCommand struct {
	Priority int
	Name     string
	Command  Command
}

type RegisteredDevices struct {
	Probe     OrderedProbingCommand
	Factories Factories
}

var (
	// Regular devices should use this probe as it is the standard
	// probing method.
	// Force higher priority than any >=0 int.
	RegularProbe = OrderedProbingCommand{
		Priority: -1,
		Name:     "regular",
		Command:  cmdInfo,
	}

	registeredDevices = []RegisteredDevices{}
)

// To be called by each device that needs to be detected.
// Can be called at package init time (see rtc and eeprom).
func RegisterDevices(devices RegisteredDevices) {
	// Find if probe is already registered
	var f *Factories
	for _, d := range registeredDevices {
		if d.Probe == devices.Probe {
			f = &d.Factories
			break
		}
	}

	if f == nil {
		// FIXME: directly insert at sorted position instead of append + sort (would be easier with newer go version)
		registeredDevices = append(registeredDevices, devices)
		sort.SliceStable(registeredDevices, func(i, j int) bool { return registeredDevices[i].Probe.Priority < registeredDevices[j].Probe.Priority })
	} else {
		// Copy factories into already registered device factories
		for k, v := range devices.Factories {
			(*f)[k] = v
		}
	}
}

// FIXME?: N64Brew seems to suggest that there can be a theoretical RTC + EEPROM combo
// here, we only report a single joybus device
// If that need to change, maybe we could return []{DeviceID, *Factory} and try all probes before returning.
func Probe(c Controller, l logger.Logger) (DeviceID, *Factory, error) {
	for _, p := range registeredDevices {
		logger.Log(l, "Probing for %s Joybus devices: ", p.Probe.Name)
		devID, _, err := Info(c, p.Probe.Command)

		if err == nil {
			logger.Log(l, "deviceID=%04x\n", devID)
		} else if c.Unfreeze(err) {
			// We didn't get any response from joybus device, unfreeze the 64drive/SI device
			// and try next probing method
			logger.Log(l, "no response\n")
			continue
		} else {
			logger.Log(l, "error: %v\n", err)
			return 0, nil, err
		}

		// Try next probing method if we get a Zero DeviceID
		if devID == 0x0000 {
			continue
		}

		if device, ok := p.Factories[devID]; ok {
			// Found supported device
			return devID, &device, nil
		} else {
			// Found unsupported device
			return devID, nil, nil
		}
	}

	// No device found
	return 0, nil, nil
}
