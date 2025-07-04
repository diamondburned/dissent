package embed

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"html"
	"net/url"
	"path/filepath"

	"github.com/diamondburned/chatkit/components/progress"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/pkg/errors"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// Opts contains options for Embed.
type Opts struct {
	// Type is the embed type. Default is Image.
	Type EmbedType
	// Provider is the image provider to use. Default is HTTPProvider.
	Provider imgutil.Provider
	// Whole, if true, will make errors show in its full information instead of
	// being hidden behind an error icon. Use this for messages only.
	Whole bool
	// CanHide, if true, will make the image hide itself on error. Use this for
	// anything not important, like embeds.
	CanHide bool
	// IgnoreWidth, if true, will cause Embed to be initialized without ever
	// setting a width request. This has the benefit of allowing the Embed to be
	// shrunken to any width, but it will introduce letterboxing.
	IgnoreWidth bool
	// Autoplay, if true, will cause the video to autoplay. For GIFs and
	// GIFVs, the user won't have to hover over the image to play it.
	Autoplay bool
	// Tooltip, if true, will cause the embed to show a tooltip when hovered.
	// If the embed errors out, a tooltip will be shown regardless.
	Tooltip bool
}

// Embed is a user-clickable image with an open callback.
//
// Widget hierarchy:
//
//   - Widgetter (?)
//   - Button
//   - Thumbnail
type Embed struct {
	*adw.Bin
	Button struct {
		*gtk.Button
		Overlay struct {
			*gtk.Overlay
			Thumbnail *onlineimage.Picture // child
			// Progress  *gtk.ProgressBar
			PlayIcon  *gtk.Image
			TypeLabel *gtk.Label // for GIFVs and GIFs
		}
	}

	name          string
	url           string
	played        bool
	playing       bool
	playCallback  func(play bool) // per-type functionality
	clickOverride func()          // override the default click behavior

	// wantMediaFile, if true, will cause SetFromURL to set the MediaFile
	// if there isn't already any.
	wantMediaFile bool

	// TODO: implement this
	// See: https://github.com/GeopJr/Tuba/pull/925
	// See: https://gitlab.gnome.org/GNOME/gtk/-/merge_requests/7186
	// wantMediaFileAsStream   bool

	// wantedMediaFile is the MediaFile that we've requested.
	wantedMediaFile *gtk.MediaFile

	curSize [2]int
	maxSize [2]int
	opts    Opts

	ctx context.Context
}

type extraImageEmbed struct{}

func (*extraImageEmbed) extra() {}

type extraGIFEmbed struct {
	anim *onlineimage.AnimationController
}

func (*extraGIFEmbed) extra() {}

var embedCSS = cssutil.Applier("thumbnail-embed", `
	.thumbnail-embed {
		padding: 0;
		margin:  0;
		/* margin-left: -2px; */
		/* border:  2px solid transparent; */
		transition-duration: 150ms;
		transition-property: all;
	}
	.thumbnail-embed,
	.thumbnail-embed:hover {
		background: none;
	}
	.thumbnail-embed .thumbnail-embed-image {
		background-color: black;
		border-radius: inherit;
		transition: linear 50ms filter;
	}
	.thumbnail-embed-errorlabel {
		color: @error_color;
		padding: 4px;
	}
	.thumbnail-embed-play {
		color: white;
		background-color: alpha(black, 0.75);
		border-radius: 999px;
		padding: 8px;
	}
	.thumbnail-embed:hover  .thumbnail-embed-play,
	.thumbnail-embed:active .thumbnail-embed-play {
		background-color: @theme_selected_bg_color;
	}
	.thumbnail-embed-gifmark {
		background-color: alpha(white, 0.85);
		color: black;
		padding: 0px 4px;
		margin:  4px;
		border-radius: 8px;
		font-weight: bold;
	}
	.message-normalembed-body:not(:only-child) {
		margin-right: 6px;
	}
	.thumbnail-embed .progress-bar {
		margin-top: 8px;
		border-radius: 4px 4px 0 0;
		color: alpha(white, 0.75);
		background: alpha(black, 0.5);
	}
	.thumbnail-embed .progress-label {
		margin: 4px;
	}
`)

// New creates a thumbnail Embed.
func New(ctx context.Context, maxW, maxH int, opts Opts) *Embed {
	if opts.Provider == nil {
		opts.Provider = imgutil.HTTPProvider
	}

	e := &Embed{
		maxSize: [2]int{maxW, maxH},
		opts:    opts,
		ctx:     ctx,
	}

	ctx = imgutil.WithOpts(ctx,
		imgutil.WithErrorFn(e.onError),
		// imgutil.WithRescale(maxW, maxH),
	)

	e.Bin = adw.NewBin()
	e.Bin.AddCSSClass("thumbnail-embed-bin")

	e.Button.Button = gtk.NewButton()
	e.Button.SetHasFrame(false)
	e.Button.SetOverflow(gtk.OverflowHidden)
	e.Button.ConnectClicked(e.activate)
	embedCSS(e.Button)
	// bindHoverPointer(e.Button.Button)

	e.Button.Overlay.Overlay = gtk.NewOverlay()
	e.Button.Overlay.AddCSSClass("thumbnail-embed-overlay")

	e.Button.Overlay.Thumbnail = onlineimage.NewPicture(ctx, opts.Provider)
	e.Button.Overlay.Thumbnail.AddCSSClass("thumbnail-embed-image")
	e.Button.Overlay.Thumbnail.SetCanShrink(true)
	e.Button.Overlay.Thumbnail.SetContentFit(gtk.ContentFitContain)

	// e.Button.Overlay.Progress = gtk.NewProgressBar()
	// e.Button.Overlay.Progress.AddCSSClass("progress-bar")
	// e.Button.Overlay.Progress.SetShowText(true)
	// e.Button.Overlay.Progress.SetVisible(false)

	e.Button.Overlay.PlayIcon = gtk.NewImageFromIconName("media-playback-start-symbolic")
	e.Button.Overlay.PlayIcon.AddCSSClass("thumbnail-embed-play")
	e.Button.Overlay.PlayIcon.SetIconSize(gtk.IconSizeNormal)
	e.Button.Overlay.PlayIcon.SetHAlign(gtk.AlignCenter)
	e.Button.Overlay.PlayIcon.SetVAlign(gtk.AlignCenter)
	e.Button.Overlay.PlayIcon.SetCanTarget(false)
	e.Button.Overlay.PlayIcon.SetVisible(false)

	e.Button.Overlay.TypeLabel = gtk.NewLabel("")
	e.Button.Overlay.TypeLabel.AddCSSClass("thumbnail-embed-gifmark")
	e.Button.Overlay.TypeLabel.SetCanTarget(false)
	e.Button.Overlay.TypeLabel.SetVAlign(gtk.AlignStart) // top
	e.Button.Overlay.TypeLabel.SetHAlign(gtk.AlignEnd)   // right
	e.Button.Overlay.TypeLabel.SetVisible(false)

	e.Button.Overlay.SetChild(e.Button.Overlay.Thumbnail)
	// e.Button.Overlay.AddOverlay(e.Button.Overlay.Progress)
	e.Button.Overlay.AddOverlay(e.Button.Overlay.PlayIcon)
	e.Button.Overlay.AddOverlay(e.Button.Overlay.TypeLabel)

	e.Button.SetChild(e.Button.Overlay)
	e.Bin.SetChild(e.Button)

	if !opts.Autoplay {
		e.enablePlayOnHover()
	}

	switch opts.Type {
	case EmbedTypeImage:
		e.AddCSSClass("thumbnail-embed-typeimage")
	case EmbedTypeAudio:
		e.AddCSSClass("thumbnail-embed-interactive")
		e.AddCSSClass("thumbnail-embed-typeaudio")

	case EmbedTypeVideo:
		e.AddCSSClass("thumbnail-embed-interactive")
		e.AddCSSClass("thumbnail-embed-typevideo")
		e.Button.Overlay.PlayIcon.SetVisible(true)

		e.extra = &extraVideoEmbed{
			progress: progress,
			loaded: func(vi *extraVideoEmbed) {
				video := gtk.NewVideo()
				video.AddCSSClass("thumbnail-embed-video")
				video.SetLoop(e.opts.Type.IsLooped())
				video.SetAutoplay(e.opts.Autoplay)
				video.SetMediaStream(vi.media)

				mediaRef := coreglib.NewWeakRef(vi.media)
				video.ConnectUnmap(func() {
					media := mediaRef.Get()
					media.Ended()
				})

				videoRef := coreglib.NewWeakRef(video)
				video.ConnectDestroy(func() {
					video := videoRef.Get()
					video.SetMediaStream(nil)
				})

				vi.media.Play()

				// Override child with the actual Video. The user won't be
				// seeing the thumbnail anymore.
				e.Bin.SetChild(video)
			},
		}

	case EmbedTypeGIFV:
		e.AddCSSClass("thumbnail-embed-interactive")
		e.AddCSSClass("thumbnail-embed-typegifv")

		// GIFVs start playing on hover, so we don't use the play button.
		// Instead, we use the GIFV label and the progress bar.
		e.Button.Overlay.TypeLabel.SetVisible(true)
		e.Button.Overlay.TypeLabel.SetLabel("GIFV")

		onlineFile := gio.NewFileForURI()
		e.wantedMediaFile = gtk.NewMediaFileForFile()

		playing := opts.Autoplay

		vi := &extraVideoEmbed{
			progress: progress,
			// This sets playing right after the media is loaded.
			// It's to prevent playing when the user already stopped
			// hovering over the thumbnail.
			loaded: func(vi *extraVideoEmbed) {
				mediaRef := coreglib.NewWeakRef(vi.media)

				e.Thumbnail.SetPaintable(vi.media)
				e.Thumbnail.ConnectUnmap(func() {
					media := mediaRef.Get()
					media.Ended()
				})

				vi.media.SetPlaying(playing)
			},
		}
		e.extra = vi

		if !opts.Autoplay {
			gif := newTypeLabel(true)
			overlay.AddOverlay(gif)

			bindPlaybackToButtonHover(e.Button, opts, func(play bool) {
				playing = play
				gif.SetVisible(!play)

				if vi.media != nil {
					// This sets playing when the media has already been
					// loaded.
					if play {
						vi.media.Play()
					} else {
						vi.media.Pause()
						vi.media.Seek(0)
					}
				} else {
					e.Thumbnail.Disable()
					e.activate()
				}
			})
		}

	case EmbedTypeGIF:
		e.AddCSSClass("thumbnail-embed-typegif")
		e.AddCSSClass("thumbnail-embed-interactive")
		e.Button.Overlay.TypeLabel.SetVisible(true)

		animation := e.Button.Overlay.Thumbnail.EnableAnimation()
		e.playCallback = func(play bool) {
			if play {
				animation.Start()
			} else {
				animation.Stop()
			}
			// Show or hide the GIF icon while it's playing.
			e.Button.Overlay.TypeLabel.SetVisible(!play)
		}
	}

	e.NotifyImage(func() {
		if paintable := e.Button.Overlay.Thumbnail.Paintable(); paintable != nil {
			e.setSize(paintable.IntrinsicWidth(), paintable.IntrinsicHeight())
			e.finishSetting()
		}
	})

	return e
}

// enablePlayOnHover adds a hover controller to the button that will
// automatically play the embed when the user hovers over it, and pause it
// when the user stops hovering over it.
func (e *Embed) enablePlayOnHover() {
	var windowUnmap func()

	button := e.Button
	button.ConnectMap(func() {
		// Bind playback to the button hover state.
		motion := gtk.NewEventControllerMotion()
		motion.NotifyProperty("contains-pointer", func() {
			if motion.ContainsPointer() {
				e.Play()
			} else {
				e.Pause()
			}
		})
		button.AddController(motion)

		// Automatically stop playback on window unfocus.
		window := button.Root().CastType(gtk.GTypeWindow).(*gtk.Window)
		windowSignal := window.NotifyProperty("is-active", func() {
			if !window.IsActive() {
				e.Pause()
			}
		})
		windowUnmap = func() {
			window.HandlerDisconnect(windowSignal)
		}
	})

	button.ConnectUnmap(func() {
		windowUnmap()
		windowUnmap = nil

		e.Pause()
	})
}

// bindHoverPointer binds the button so that the cursor changes to a pointer
// cursor on hover.
func bindHoverPointer(button *gtk.Button) {
	buttonRef := coreglib.NewWeakRef(button)

	motion := gtk.NewEventControllerMotion()
	motion.ConnectEnter(func(x, y float64) {
		button := buttonRef.Get()
		button.SetCursorFromName("pointer")
	})
	motion.ConnectLeave(func() {
		button := buttonRef.Get()
		button.SetCursor(nil)
	})

	button.AddController(motion)
}

// SetHAlign sets the horizontal alignment of the embed relative to its parent.
func (e *Embed) SetHAlign(align gtk.Align) {
	e.Bin.SetHAlign(align)
	e.Button.SetHAlign(align)
}

// SetName sets the given embed name into everything that's displaying the embed
// name.
func (e *Embed) SetName(name string) {
	e.name = name
	if e.opts.Tooltip {
		e.Button.SetTooltipText(name)
	}
}

// URL returns the Embed's current URL.
func (e *Embed) URL() string {
	return e.url
}

// SetFromURL sets the URL of the thumbnail embed.
func (e *Embed) SetFromURL(url string) {
	e.url = url

	if e.opts.Type == 0 {
		e.opts.Type = TypeFromURL(url)
	}

	if e.wantMediaFile {
		file := gio.NewFileForURI(url)
		e.wantedMediaFile = gtk.NewMediaFileForFile(file)
	}

	switch e.opts.Type {
	case EmbedTypeImage, EmbedTypeGIF:
		e.Button.Overlay.Thumbnail.SetURL(url)
	default:
		e.Button.Overlay.Thumbnail.Disable()
	}

	if e.opts.Autoplay {
		e.Play()
	}

	// switch embedType := TypeFromURL(url); embedType {
	// case EmbedTypeImage, EmbedTypeGIF:
	// 	e.Thumbnail.SetURL(url)
	//
	// 	if embedType == EmbedTypeGIF && e.opts.Autoplay {
	// 		gif := e.extra.(*extraGIFEmbed)
	// 		gif.anim.Start()
	// 	}
	//
	// case EmbedTypeVideo, EmbedTypeGIFV:
	// 	e.Thumbnail.Disable()
	//
	// 	if e.opts.Autoplay {
	// 		vi := e.extra.(*extraVideoEmbed)
	// 		vi.downloadVideo(e)
	// 	}
	//
	// case EmbedTypeAudio:
	// 	e.Thumbnail.Disable()
	// }
}

// NotifyImage calls f everytime the Embed thumbnail changes.
func (e *Embed) NotifyImage(f func()) glib.SignalHandle {
	return e.Thumbnail.NotifyProperty("paintable", f)
}

// undo effects
func (e *Embed) finishSetting() {
	if e.opts.CanHide {
		e.SetVisible(true)
	}

	if e.opts.Whole {
		e.Button.SetChild(e.Thumbnail)
	}
}

func (e *Embed) onError(err error) {
	if e.opts.CanHide {
		e.SetVisible(false)
		return
	}

	if e.opts.Whole {
		// Mild annoyance: the padding of this label actually grows the image a
		// bit. Not sure how to fix it.
		errLabel := gtk.NewLabel("Error fetching image: " + html.EscapeString(err.Error()))
		errLabel.AddCSSClass("mcontent-image-errorlabel")
		errLabel.SetEllipsize(pango.EllipsizeEnd)
		errLabel.SetWrap(true)
		errLabel.SetWrapMode(pango.WrapWordChar)
		errLabel.SetLines(2)
		e.Button.SetChild(errLabel)
	} else {
		size := e.curSize
		if size == [2]int{} {
			// No size; pick the max size.
			size = e.maxSize
		}
		iconMissing := imgutil.IconPaintable("image-missing", size[0], size[1])
		e.Thumbnail.SetPaintable(iconMissing)
	}

	var tooltip string
	if e.opts.Tooltip && e.name != "" {
		tooltip += html.EscapeString(e.name) + "\n"
	}
	tooltip += "<b>Error:</b> " + html.EscapeString(err.Error())
	e.Button.SetTooltipMarkup(tooltip)
}

func (e *Embed) isBusy() bool {
	base := gtk.BaseWidget(e)
	return !base.IsSensitive()
}

func (e *Embed) setBusy(busy bool) {
	gtk.BaseWidget(e).SetSensitive(!busy)
}

type extraVideoEmbed struct {
	progress *progress.Bar
	media    *gtk.MediaFile
	loaded   func(*extraVideoEmbed)
}

func (*extraVideoEmbed) extra() {}

func (vi *extraVideoEmbed) downloadVideo(e *Embed) {
	if e.isBusy() || vi.media != nil {
		return
	}

	vi.progress.SetVisible(true)
	if e.url == "" {
		vi.progress.Error(errors.New("video has no URL"))
		return
	}

	e.setBusy(true)
	cleanup := func() { e.setBusy(false) }

	ctx := e.ctx

	u, err := url.Parse(e.url)
	if err != nil {
		vi.progress.Error(errors.Wrap(err, "invalid URL"))
		cleanup()
		return
	}

	gtkutil.Async(ctx, func() func() {
		var file string

		switch u.Scheme {
		case "http", "https":
			cacheDir := app.FromContext(ctx).CachePath("videos")
			cacheDst := urlPath(cacheDir, u.String())
			if !fetchURL(ctx, u.String(), cacheDst, vi.progress) {
				return cleanup
			}
			file = cacheDst
		case "file":
			file = u.Host + u.Path
		default:
			return func() {
				vi.progress.Error(fmt.Errorf("unknown scheme %q (go do the refactor!)", u.Scheme))
				cleanup()
			}
		}

		return func() {
			cleanup()
			vi.progress.SetVisible(false)

			media := gtk.NewMediaFileForFilename(file)
			media.SetLoop(e.opts.Type.IsLooped())
			media.SetMuted(e.opts.Type.IsMuted())
			vi.media = media

			vi.loaded(vi)
			vi.loaded = nil
		}
	})
}

// SetOpenURL sets the callback to be called when the user clicks the image.
func (e *Embed) SetOpenURL(f func()) {
	e.click = f
}

func (e *Embed) activate() {
	if e.click != nil {
		e.click()
		return
	}

	e.ActivateDefault()
}

// ActivateDefault triggers the default function that's called by default by
// SetOpenURL.
func (e *Embed) ActivateDefault() {
	switch e.opts.Type {
	case EmbedTypeVideo, EmbedTypeGIFV:
		vi := e.extra.(*extraVideoEmbed)
		vi.downloadVideo(e)
	default:
		app.OpenURI(e.ctx, e.url)
	}
}

// setPlay is a GTK-style setter for [Play] and [Pause].
func (e *Embed) setPlay(play bool) {
	if play {
		e.Play()
	} else {
		e.Pause()
	}
}

// Play starts the playback of the embed. This is only used for Autoplay.
// Calling this before [SetFromURL] will do nothing.
// Calling this on an embed of type [EmbedTypeImage] will do nothing.
func (e *Embed) Play() {
	if e.url == "" || e.isBusy() || e.played {
		return
	}

	if e.wantMediaFile {
		gtk.NewMediaFileForInputStream()
		e.wantedMediaFile = nil
	}

	switch e.opts.Type {
	case EmbedTypeGIF:
	}

	if !e.playing && e.playCallback != nil {
		e.playCallback(true)
	}

	e.playing = true
	e.played = true
}

// Pause pauses the playback of the embed.
func (e *Embed) Pause() {
	if e.isBusy() || !e.played {
		return
	}

	if e.playing && e.playCallback != nil {
		e.playCallback(false)
	}

	e.playing = false
}

// SetMaxSize sets the maximum size of the image.
func (e *Embed) SetMaxSize(w, h int) {
	e.maxSize = [2]int{w, h}
}

// ShrinkMaxSize sets the maximum size of the image to be the smaller of the
// current maximum size and the given size.
func (e *Embed) ShrinkMaxSize(w, h int) {
	w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	e.SetMaxSize(w, h)
}

// SetSizeRequest sets the minimum size of a widget. The dimensions are clamped
// to the maximum size given during construction, if any.
func (e *Embed) SetSizeRequest(w, h int) {
	if e.maxSize != [2]int{} {
		w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	}
	if e.opts.IgnoreWidth {
		w = -1
	}
	e.Bin.SetSizeRequest(w, h)
}

// setSize sets the size of the image embed.
func (e *Embed) setSize(w, h int) {
	if e.maxSize != [2]int{} {
		w, h = imgutil.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	}

	e.curSize = [2]int{w, h}

	if e.opts.IgnoreWidth {
		w = -1
	}
	e.Bin.SetSizeRequest(w, h)
}

// Size returns the original embed size optionally scaled down, or 0 if no
// images have been fetched yet or if SetSize has never been called before.
func (e *Embed) Size() (w, h int) {
	return e.curSize[0], e.curSize[1]
}

func urlPath(baseDir, url string) string {
	b := sha1.Sum([]byte(url))
	f := base64.URLEncoding.EncodeToString(b[:])
	return filepath.Join(baseDir, f)
}
