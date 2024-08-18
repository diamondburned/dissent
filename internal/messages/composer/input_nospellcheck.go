//go:build nospellcheck

package composer

import (
	"log/slog"
	"runtime"
)

func hookSpellChecker(i *Input) {
	slog.Debug(
		"spell checking is disabled at build time",
		"goos", runtime.GOOS)
}
