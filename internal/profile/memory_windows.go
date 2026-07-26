//go:build windows

package profile

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func totalMemory() uint64 {
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel.NewProc("GlobalMemoryStatusEx")
	ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return 0
	}
	return status.TotalPhys
}

func storageAvailable(path string) uint64 {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	var available uint64
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel.NewProc("GetDiskFreeSpaceExW")
	ok, _, _ := proc.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&available)),
		0,
		0,
	)
	if ok == 0 {
		return 0
	}
	return available
}
