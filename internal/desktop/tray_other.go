//go:build desktop && !darwin && !windows && !linux

package desktop

type noopTray struct{}

func newTrayController() trayController { return &noopTray{} }
func (*noopTray) Start(*App)            {}
func (*noopTray) Update(Snapshot)       {}
func (*noopTray) SetVisible(bool)       {}
func (*noopTray) Stop()                 {}
