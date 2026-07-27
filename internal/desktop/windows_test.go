package desktop

import (
	"strings"
	"testing"
)

func TestDesktopLoginDefinitionsKeepWindowAndMenuBarIndependent(t *testing.T) {
	executable := `/Applications/Spare & Tools/Spare`
	hiddenMac := darwinDesktopLoginContent(executable, false)
	if !strings.Contains(hiddenMac, "Spare &amp; Tools") ||
		!strings.Contains(hiddenMac, "<string>--hidden</string>") {
		t.Fatalf("hidden macOS definition = %s", hiddenMac)
	}
	visibleMac := darwinDesktopLoginContent(executable, true)
	if strings.Contains(visibleMac, "--hidden") {
		t.Fatalf("visible macOS definition starts hidden: %s", visibleMac)
	}

	hiddenWindows := windowsDesktopLoginCommand(`C:\Spare Tools\Spare.exe`, false)
	if hiddenWindows != `"C:\Spare Tools\Spare.exe" --hidden` {
		t.Fatalf("hidden Windows command = %q", hiddenWindows)
	}
	visibleWindows := windowsDesktopLoginCommand(`C:\Spare Tools\Spare.exe`, true)
	if visibleWindows != `"C:\Spare Tools\Spare.exe"` {
		t.Fatalf("visible Windows command = %q", visibleWindows)
	}

	hiddenLinux := linuxDesktopLoginContent("/opt/Spare Tools/Spare", false)
	if !strings.Contains(hiddenLinux, `Exec="/opt/Spare Tools/Spare" --hidden`) {
		t.Fatalf("hidden Linux definition = %s", hiddenLinux)
	}
	visibleLinux := linuxDesktopLoginContent("/opt/Spare Tools/Spare", true)
	if strings.Contains(visibleLinux, "--hidden") {
		t.Fatalf("visible Linux definition starts hidden: %s", visibleLinux)
	}
}
