//go:build desktop && windows

package desktop

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowsTrayMessage = 0x8001

	wmCommand     = 0x0111
	wmClose       = 0x0010
	wmDestroy     = 0x0002
	wmContextMenu = 0x007B
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	mfString    = 0x00000000
	mfDisabled  = 0x00000002
	mfSeparator = 0x00000800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	cmdOpenRecipe = 1001
	cmdShare      = 1002
	cmdToggle     = 1003
	cmdActivity   = 1004
	cmdOpenSpare  = 1005
	cmdQuit       = 1006
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostMessageW       = user32.NewProc("PostMessageW")
	procPostQuitMessage    = user32.NewProc("PostQuitMessage")
	procLoadIconW          = user32.NewProc("LoadIconW")
	procCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	procAppendMenuW        = user32.NewProc("AppendMenuW")
	procDestroyMenu        = user32.NewProc("DestroyMenu")
	procTrackPopupMenu     = user32.NewProc("TrackPopupMenu")
	procSetForegroundWnd   = user32.NewProc("SetForegroundWindow")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
	procShellNotifyIconW   = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	windowsTrayCallback    = syscall.NewCallback(windowsTrayWindowProc)
	windowsTrayRegistry    = map[uintptr]*windowsTray{}
	windowsTrayRegistryMux sync.RWMutex
)

type windowsPoint struct {
	X int32
	Y int32
}

type windowsMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   windowsPoint
}

type windowsClass struct {
	Size            uint32
	Style           uint32
	WindowProcedure uintptr
	ClassExtra      int32
	WindowExtra     int32
	Instance        uintptr
	Icon            uintptr
	Cursor          uintptr
	Background      uintptr
	MenuName        *uint16
	ClassName       *uint16
	SmallIcon       uintptr
}

type windowsNotifyIconData struct {
	Size             uint32
	Window           uintptr
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	ItemGUID         windows.GUID
	BalloonIcon      uintptr
}

type windowsTray struct {
	app *App

	mu       sync.RWMutex
	window   uintptr
	icon     uintptr
	visible  bool
	snapshot Snapshot
	ready    chan struct{}
}

func newTrayController() trayController {
	return &windowsTray{
		visible: true,
		ready:   make(chan struct{}),
	}
}

func (t *windowsTray) Start(app *App) {
	t.app = app
	go t.run()
	<-t.ready
}

func (t *windowsTray) Update(snapshot Snapshot) {
	t.mu.Lock()
	t.snapshot = snapshot
	t.mu.Unlock()
	t.updateIcon(nimModify)
}

func (t *windowsTray) SetVisible(visible bool) {
	t.mu.Lock()
	changed := t.visible != visible
	t.visible = visible
	t.mu.Unlock()
	if !changed {
		return
	}
	if visible {
		t.updateIcon(nimAdd)
	} else {
		t.updateIcon(nimDelete)
	}
}

func (t *windowsTray) Stop() {
	t.mu.RLock()
	window := t.window
	t.mu.RUnlock()
	if window != 0 {
		procPostMessageW.Call(window, wmClose, 0, 0)
	}
}

func (t *windowsTray) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("SpareDesktopTrayWindow")
	class := windowsClass{
		Size:            uint32(unsafe.Sizeof(windowsClass{})),
		WindowProcedure: windowsTrayCallback,
		Instance:        instance,
		ClassName:       className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	window, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0,
		0, 0, 0, 0,
		^uintptr(2), // HWND_MESSAGE
		0,
		instance,
		0,
	)
	icon, _, _ := procLoadIconW.Call(0, 32512) // IDI_APPLICATION

	t.mu.Lock()
	t.window = window
	t.icon = icon
	t.mu.Unlock()
	windowsTrayRegistryMux.Lock()
	windowsTrayRegistry[window] = t
	windowsTrayRegistryMux.Unlock()
	close(t.ready)

	if window == 0 {
		return
	}
	t.updateIcon(nimAdd)

	var message windowsMessage
	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}

	t.updateIcon(nimDelete)
	windowsTrayRegistryMux.Lock()
	delete(windowsTrayRegistry, window)
	windowsTrayRegistryMux.Unlock()
}

func (t *windowsTray) updateIcon(operation uintptr) {
	t.mu.RLock()
	window := t.window
	icon := t.icon
	visible := t.visible
	snapshot := t.snapshot
	t.mu.RUnlock()
	if window == 0 || (operation != nimDelete && !visible) {
		return
	}
	status := "Spare · No active job"
	if len(snapshot.Instances) > 0 {
		instance := snapshot.Instances[0]
		title := instance.RecipeID
		for _, recipe := range snapshot.Recipes {
			if recipe.ID == instance.RecipeID {
				title = recipe.Title
				break
			}
		}
		status = "Spare · " + title + " · " + string(instance.Status)
	}
	data := windowsNotifyIconData{
		Size:            uint32(unsafe.Sizeof(windowsNotifyIconData{})),
		Window:          window,
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: windowsTrayMessage,
		Icon:            icon,
	}
	copy(data.Tip[:], windows.StringToUTF16(status))
	procShellNotifyIconW.Call(operation, uintptr(unsafe.Pointer(&data)))
}

func windowsTrayWindowProc(window uintptr, message uint32, wParam, lParam uintptr) uintptr {
	windowsTrayRegistryMux.RLock()
	tray := windowsTrayRegistry[window]
	windowsTrayRegistryMux.RUnlock()
	switch message {
	case windowsTrayMessage:
		if lParam == wmRButtonUp || lParam == wmLButtonUp || lParam == wmContextMenu {
			if tray != nil {
				tray.showMenu()
			}
		}
		return 0
	case wmCommand:
		if tray != nil {
			tray.runCommand(uint16(wParam & 0xffff))
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(window)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(window, uintptr(message), wParam, lParam)
	return result
}

func (t *windowsTray) showMenu() {
	t.mu.RLock()
	snapshot := t.snapshot
	window := t.window
	t.mu.RUnlock()
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	appendWindowsMenu(menu, 0, "Spare", false)
	status := "No active job"
	openLabel := "Choose a job"
	toggleLabel := ""
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
		if instance.DesiredState == "running" {
			toggleLabel = "Pause " + title
		} else {
			toggleLabel = "Start " + title
		}
	}
	appendWindowsMenu(menu, 0, status, false)
	appendWindowsSeparator(menu)
	appendWindowsMenu(menu, cmdOpenRecipe, openLabel, true)
	if len(snapshot.Instances) > 0 {
		appendWindowsMenu(menu, cmdShare, "Show QR", true)
		appendWindowsMenu(menu, cmdToggle, toggleLabel, true)
		appendWindowsMenu(menu, cmdActivity, "Recent activity", true)
	}
	appendWindowsMenu(menu, cmdOpenSpare, "Open Spare", true)
	appendWindowsSeparator(menu)
	appendWindowsMenu(menu, cmdQuit, "Quit Spare", true)

	var point windowsPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	procSetForegroundWnd.Call(window)
	command, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmReturnCmd,
		uintptr(point.X),
		uintptr(point.Y),
		0,
		window,
		0,
	)
	if command != 0 {
		t.runCommand(uint16(command))
	}
}

func (t *windowsTray) runCommand(command uint16) {
	action := ""
	switch command {
	case cmdOpenRecipe:
		t.mu.RLock()
		hasInstance := len(t.snapshot.Instances) > 0
		t.mu.RUnlock()
		if hasInstance {
			action = "open_recipe"
		} else {
			action = "choose"
		}
	case cmdShare:
		action = "share"
	case cmdToggle:
		action = "toggle"
	case cmdActivity:
		action = "activity"
	case cmdOpenSpare:
		action = "open_spare"
	case cmdQuit:
		action = "quit"
	}
	if action != "" && t.app != nil {
		go t.app.handleTrayAction(action)
	}
}

func appendWindowsMenu(menu uintptr, command uint16, label string, enabled bool) {
	flags := uintptr(mfString)
	if !enabled {
		flags |= mfDisabled
	}
	text, _ := windows.UTF16PtrFromString(label)
	procAppendMenuW.Call(menu, flags, uintptr(command), uintptr(unsafe.Pointer(text)))
}

func appendWindowsSeparator(menu uintptr) {
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
}
