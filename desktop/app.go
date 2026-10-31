package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync/atomic"

	"conduit/daemon/launch"
	"fyne.io/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the native window around the daemon page.
type App struct {
	ctx      context.Context
	daemon   *launch.App
	quitting int32
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.daemon != nil {
		a.daemon.SetShowHandler(a.show)
		a.daemon.SetQuitHandler(a.requestQuit)
	}
	target := fmt.Sprintf("http://127.0.0.1:%d/?shell=app", a.port())
	quoted, _ := json.Marshal(target)
	wruntime.WindowExecJS(ctx, "window.__conduitTarget = "+string(quoted)+"; if (window.conduitGo) window.conduitGo();")
	go systray.Run(a.onTrayReady, func() {})
}

func (a *App) port() int {
	if a.daemon == nil {
		return launch.DefaultHTTPPort
	}
	return a.daemon.Port()
}

func (a *App) show() {
	if a.ctx == nil {
		return
	}
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowShow(a.ctx)
	// Windows keeps a window that a background process shows behind the
	// browser. Toggling always-on-top lifts it to the front without waiting
	// for the OS to hand over focus.
	wruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wruntime.WindowSetAlwaysOnTop(a.ctx, false)
}

func (a *App) beforeClose(ctx context.Context) bool {
	if atomic.LoadInt32(&a.quitting) == 1 {
		return false
	}
	wruntime.WindowHide(ctx)
	return true
}

func (a *App) requestQuit() {
	atomic.StoreInt32(&a.quitting, 1)
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

func (a *App) shutdown(ctx context.Context) {
	systray.Quit()
	if a.daemon != nil {
		_ = a.daemon.Shutdown(ctx)
	}
}

func (a *App) onTrayReady() {
	systray.SetTitle("Conduit")
	systray.SetTooltip("Conduit")
	if runtime.GOOS == "windows" && len(iconICO) > 0 {
		systray.SetIcon(iconICO)
	} else if len(iconPNG) > 0 {
		systray.SetIcon(iconPNG)
	}
	// Left-click opens the window; right-click shows the menu.
	systray.SetOnTapped(a.show)
	openItem := systray.AddMenuItem("Open", "Show Conduit")
	installItem := systray.AddMenuItem("Install extension", "Open the extension guide")
	quitItem := systray.AddMenuItem("Quit", "Quit Conduit")
	go func() {
		for {
			select {
			case <-openItem.ClickedCh:
				a.show()
			case <-installItem.ClickedCh:
				if a.daemon != nil {
					_ = a.daemon.OpenExtensionGuide()
				}
			case <-quitItem.ClickedCh:
				a.requestQuit()
				return
			}
		}
	}()
}
