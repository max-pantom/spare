//go:build windows

package profile

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

func portableTraits() (bool, bool) {
	var status systemPowerStatus
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel.NewProc("GetSystemPowerStatus")
	ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&status)))
	hasBattery := ok != 0 && status.BatteryFlag != 128
	return hasBattery, false
}
