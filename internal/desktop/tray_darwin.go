//go:build desktop && darwin

package desktop

/*
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#include <stdlib.h>

void spare_tray_start(void);
void spare_tray_update(const char *headline, const char *status, const char *openLabel, const char *toggleLabel, int hasInstance, int isDrop, int hasAddress, int needsAttention, int canReconnect, int iconState);
void spare_tray_set_visible(int visible);
void spare_tray_stop(void);
*/
import "C"

import (
	"sync"
	"unsafe"
)

type darwinTray struct {
	app *App
}

var (
	activeTray   *darwinTray
	activeTrayMu sync.RWMutex
)

func newTrayController() trayController {
	return &darwinTray{}
}

func (t *darwinTray) Start(app *App) {
	t.app = app
	activeTrayMu.Lock()
	activeTray = t
	activeTrayMu.Unlock()
	C.spare_tray_start()
}

func (t *darwinTray) Update(snapshot Snapshot) {
	presentation := presentTray(snapshot)
	cHeadline := C.CString(presentation.Headline)
	cStatus := C.CString(presentation.Status)
	cOpen := C.CString(presentation.OpenLabel)
	cToggle := C.CString(presentation.ToggleLabel)
	defer C.free(unsafe.Pointer(cHeadline))
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cOpen))
	defer C.free(unsafe.Pointer(cToggle))
	C.spare_tray_update(
		cHeadline,
		cStatus,
		cOpen,
		cToggle,
		trayBool(presentation.HasInstance),
		trayBool(presentation.IsDrop),
		trayBool(presentation.Address != ""),
		trayBool(presentation.NeedsAttention),
		trayBool(presentation.CanReconnect),
		C.int(presentation.IconState),
	)
}

func (*darwinTray) SetVisible(visible bool) {
	value := 0
	if visible {
		value = 1
	}
	C.spare_tray_set_visible(C.int(value))
}

func (*darwinTray) Stop() {
	C.spare_tray_stop()
	activeTrayMu.Lock()
	activeTray = nil
	activeTrayMu.Unlock()
}

//export spareTrayAction
func spareTrayAction(action *C.char) {
	activeTrayMu.RLock()
	tray := activeTray
	activeTrayMu.RUnlock()
	if tray == nil || tray.app == nil {
		return
	}
	tray.app.handleTrayAction(C.GoString(action))
}

func trayBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}
