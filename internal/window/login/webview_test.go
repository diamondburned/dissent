//go:build linux && !nowebkit

package login

import (
	"testing"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	webkit "github.com/diamondburned/gotk4-webkitgtk/pkg/webkit/v6"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestNewEphemeralWebView(t *testing.T) {
	if !gtk.InitCheck() {
		t.Skip("GTK display not available")
	}
	adw.Init()

	ui := gtk.NewBuilderFromString(webviewUI)
	root := ui.GetObject("rootContent")
	if root == nil || ui.GetObject("webOverlay") == nil || ui.GetObject("progressBar") == nil {
		t.Fatal("webview.ui missing expected objects")
	}
	navPage, ok := root.Cast().(*adw.NavigationPage)
	if !ok || navPage.Tag() != "browser" {
		t.Fatal("expected rootContent to be an AdwNavigationPage with tag 'browser'")
	}

	view := newEphemeralWebView()
	if view == nil {
		t.Fatal("newEphemeralWebView returned nil")
	}
	if !view.NetworkSession().IsEphemeral() {
		t.Fatal("expected ephemeral network session")
	}

	ucm := view.UserContentManager()
	ucm.AddScript(webkit.NewUserScript(
		captureJS,
		webkit.UserContentInjectAllFrames,
		webkit.UserScriptInjectAtDocumentStart,
		[]string{"https://discord.com/*", "https://*.discord.com/*"},
		nil,
	))
	if !ucm.RegisterScriptMessageHandler(tokenMessageHandler, "") {
		t.Fatal("failed to register script message handler")
	}
	ucm.UnregisterScriptMessageHandler(tokenMessageHandler, "")
}
