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
	mu                  sync.RWMutex
	app                 *App
	started             bool
	hasLastPresentation bool
	lastPresentation    trayPresentation
}

var (
	activeTray   *darwinTray
	activeTrayMu sync.RWMutex
)

func newTrayController() trayController {
	return &darwinTray{}
}

func (t *darwinTray) Start(app *App) {
	t.mu.Lock()
	t.app = app
	t.started = true
	t.hasLastPresentation = false
	t.mu.Unlock()

	activeTrayMu.Lock()
	activeTray = t
	activeTrayMu.Unlock()
	C.spare_tray_start()
	t.Update(Snapshot{})
}

func (t *darwinTray) Update(snapshot Snapshot) {
	presentation := presentTray(snapshot)

	t.mu.Lock()
	if !t.started ||
		(t.hasLastPresentation && t.lastPresentation == presentation) {
		t.mu.Unlock()
		return
	}
	t.lastPresentation = presentation
	t.hasLastPresentation = true
	t.mu.Unlock()

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

func (t *darwinTray) Stop() {
	activeTrayMu.Lock()
	if activeTray == t {
		activeTray = nil
	}
	activeTrayMu.Unlock()

	t.mu.Lock()
	t.started = false
	t.app = nil
	t.hasLastPresentation = false
	t.mu.Unlock()

	C.spare_tray_stop()
}

//export spareTrayAction
func spareTrayAction(action *C.char) {
	actionName := C.GoString(action)
	activeTrayMu.RLock()
	tray := activeTray
	activeTrayMu.RUnlock()
	if tray == nil {
		return
	}

	tray.mu.RLock()
	app := tray.app
	started := tray.started
	tray.mu.RUnlock()
	if app == nil || !started {
		return
	}

	// Native menu callbacks run on Cocoa's main thread. Return control to
	// AppKit before invoking Wails or the daemon so the menu can close cleanly.
	go app.handleTrayAction(actionName)
}

func trayBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}
