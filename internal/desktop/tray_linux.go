//go:build desktop && linux

package desktop

/*
#cgo pkg-config: gtk+-3.0 ayatana-appindicator3-0.1
#include <stdlib.h>

void spare_linux_tray_start(void);
void spare_linux_tray_update(const char *headline, const char *status, const char *openLabel, const char *toggleLabel, int hasInstance, int isDrop, int hasAddress, int needsAttention, int canReconnect, int iconState);
void spare_linux_tray_set_visible(int visible);
void spare_linux_tray_stop(void);
*/
import "C"

import (
	"sync"
	"unsafe"
)

type linuxTray struct {
	app *App
}

var (
	activeLinuxTray   *linuxTray
	activeLinuxTrayMu sync.RWMutex
)

func newTrayController() trayController {
	return &linuxTray{}
}

func (t *linuxTray) Start(app *App) {
	t.app = app
	activeLinuxTrayMu.Lock()
	activeLinuxTray = t
	activeLinuxTrayMu.Unlock()
	C.spare_linux_tray_start()
}

func (t *linuxTray) Update(snapshot Snapshot) {
	presentation := presentTray(snapshot)
	cHeadline := C.CString(presentation.Headline)
	cStatus := C.CString(presentation.Status)
	cOpen := C.CString(presentation.OpenLabel)
	cToggle := C.CString(presentation.ToggleLabel)
	defer C.free(unsafe.Pointer(cHeadline))
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cOpen))
	defer C.free(unsafe.Pointer(cToggle))
	C.spare_linux_tray_update(
		cHeadline,
		cStatus,
		cOpen,
		cToggle,
		C.int(boolInt(presentation.HasInstance)),
		C.int(boolInt(presentation.IsDrop)),
		C.int(boolInt(presentation.Address != "")),
		C.int(boolInt(presentation.NeedsAttention)),
		C.int(boolInt(presentation.CanReconnect)),
		C.int(presentation.IconState),
	)
}

func (*linuxTray) SetVisible(visible bool) {
	value := 0
	if visible {
		value = 1
	}
	C.spare_linux_tray_set_visible(C.int(value))
}

func (*linuxTray) Stop() {
	C.spare_linux_tray_stop()
	activeLinuxTrayMu.Lock()
	activeLinuxTray = nil
	activeLinuxTrayMu.Unlock()
}

//export spareLinuxTrayAction
func spareLinuxTrayAction(action *C.char) {
	activeLinuxTrayMu.RLock()
	tray := activeLinuxTray
	activeLinuxTrayMu.RUnlock()
	if tray == nil || tray.app == nil {
		return
	}
	tray.app.handleTrayAction(C.GoString(action))
}
