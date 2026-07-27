//go:build desktop && darwin

package desktop

/*
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#include <stdlib.h>

void spare_tray_start(void);
void spare_tray_update(const char *status, const char *openLabel, const char *toggleLabel, int hasInstance, int running);
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
	status := "No active job"
	openLabel := "Choose a job"
	toggleLabel := ""
	hasInstance := 0
	running := 0
	if len(snapshot.Instances) > 0 {
		instance := snapshot.Instances[0]
		title := instance.RecipeID
		for _, recipe := range snapshot.Recipes {
			if recipe.ID == instance.RecipeID {
				title = recipe.Title
				break
			}
		}
		status = title + " · " + instance.Status
		openLabel = "Open " + title
		hasInstance = 1
		if instance.DesiredState == "running" {
			running = 1
			toggleLabel = "Pause " + title
		} else {
			toggleLabel = "Start " + title
		}
	}
	cStatus := C.CString(status)
	cOpen := C.CString(openLabel)
	cToggle := C.CString(toggleLabel)
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cOpen))
	defer C.free(unsafe.Pointer(cToggle))
	C.spare_tray_update(cStatus, cOpen, cToggle, C.int(hasInstance), C.int(running))
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
