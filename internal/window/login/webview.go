//go:build linux && !nowebkit

package login

import (
	"context"
	_ "embed"
	"errors"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4-webkitgtk/pkg/javascriptcore/v6"
	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const browserLoginSupported = true

const (
	discordLoginURL     = "https://discord.com/login"
	tokenMessageHandler = "dissentToken"
)

//go:embed capture.js
var captureJS string

//go:embed webview.ui
var webviewUI string

type webLoginView struct {
	webkit.WebView
}

var webLoginViewType = coreglib.RegisterSubclass[*webLoginView]()

func newEphemeralWebView() *webkit.WebView {
	session := webkit.NewNetworkSessionEphemeral()
	view := webLoginViewType.NewWithProperties(map[string]any{
		"network-session": session,
	})
	return &view.WebView
}

func (c *Component) loginBrowser() {
	if c.page.VisiblePageTag() == "browser" {
		return
	}

	c.HideError()
	c.SetBusy()

	view := newEphemeralWebView()

	ucm := view.UserContentManager()
	ucm.AddScript(webkit.NewUserScript(
		captureJS,
		webkit.UserContentInjectAllFrames,
		webkit.UserScriptInjectAtDocumentStart,
		[]string{"https://discord.com/*", "https://*.discord.com/*"},
		nil,
	))

	ui := gtk.NewBuilderFromString(webviewUI)
	get := func(name string) glib.Objector { return ui.GetObject(name).Cast() }

	navPage := get("rootContent").(*adw.NavigationPage)
	windowTitle := get("windowTitle").(*adw.WindowTitle)
	overlay := get("webOverlay").(*gtk.Overlay)
	progress := get("progressBar").(*gtk.ProgressBar)

	windowTitle.SetSubtitle(discordLoginURL)
	overlay.SetChild(view)

	finished := false
	var poppedHandle coreglib.SignalHandle
	cleanup := func() {
		if poppedHandle != 0 {
			c.page.HandlerDisconnect(poppedHandle)
			poppedHandle = 0
		}
		view.StopLoading()
		ucm.UnregisterScriptMessageHandler(tokenMessageHandler, "")
		if mgr := view.NetworkSession().WebsiteDataManager(); mgr != nil {
			mgr.Clear(context.Background(), webkit.WebsiteDataAll, 0, nil)
		}
		overlay.SetChild(nil)
	}
	finish := func(token string, err error) {
		if finished {
			return
		}
		finished = true

		cleanup()
		if c.page.VisiblePageTag() == "browser" {
			c.page.PopToTag("greeter")
		}
		c.SetDone()

		if err != nil {
			c.ShowError(err)
		} else if token != "" {
			c.loginToken(token)
		}
	}

	acceptToken := func(raw string) {
		token := strings.TrimSpace(strings.Trim(raw, `"`))
		if token == "" ||
			strings.HasPrefix(token, "Bearer ") ||
			strings.HasPrefix(token, "Basic ") {
			return
		}
		finish(token, nil)
	}

	ucm.ConnectScriptMessageReceived(func(v *javascriptcore.Value) {
		if v == nil || !v.IsString() {
			return
		}
		acceptToken(v.String())
	})
	if !ucm.RegisterScriptMessageHandler(tokenMessageHandler, "") {
		finish("", errors.New("failed to register WebKit script message handler"))
		return
	}

	// Fallback signal: inspect Authorization header on Discord API requests.
	view.ConnectResourceLoadStarted(func(_ *webkit.WebResource, req *webkit.URIRequest) {
		if req == nil || !strings.HasPrefix(req.URI(), "https://discord.com/api/") {
			return
		}
		if h := req.HTTPHeaders(); h != nil {
			if auth := h.One("Authorization"); auth != "" {
				acceptToken(auth)
			}
		}
	})

	view.NotifyProperty("estimated-load-progress", func() {
		p := view.EstimatedLoadProgress()
		progress.SetFraction(p)
		progress.SetVisible(view.IsLoading() && p < 1.0)
	})
	view.ConnectLoadChanged(func(ev webkit.LoadEvent) {
		if ev == webkit.LoadFinished {
			progress.SetVisible(false)
		}
	})
	view.ConnectWebProcessTerminated(func(reason webkit.WebProcessTerminationReason) {
		if reason == webkit.WebProcessCrashed || reason == webkit.WebProcessExceededMemoryLimit {
			finish("", errors.New("browser login process terminated unexpectedly"))
		}
	})
	view.ConnectClose(func() {
		finish("", nil)
	})
	poppedHandle = c.page.ConnectPopped(func(popped *adw.NavigationPage) {
		if popped == nil || popped.Native() != navPage.Native() {
			return
		}
		if !finished {
			finished = true
			cleanup()
			c.SetDone()
		}
	})

	view.LoadURI(discordLoginURL)
	c.page.Push(navPage)
}
