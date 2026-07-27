//go:build desktop && linux

package desktop

/*
#cgo pkg-config: gtk+-3.0 ayatana-appindicator3-0.1
#include <stdlib.h>

void spare_linux_tray_start(void);
void spare_linux_tray_update(const char *status, const char *openLabel, const char *toggleLabel, int hasInstance, int running);
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
		status = title + " · " + string(instance.Status)
		openLabel = "Open " + title
		toggleLabel = "Start " + title
		hasInstance = 1
		if instance.DesiredState == "running" {
			running = 1
			toggleLabel = "Pause " + title
		}
	}
	cStatus := C.CString(status)
	cOpen := C.CString(openLabel)
	cToggle := C.CString(toggleLabel)
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cOpen))
	defer C.free(unsafe.Pointer(cToggle))
	C.spare_linux_tray_update(cStatus, cOpen, cToggle, C.int(hasInstance), C.int(running))
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
