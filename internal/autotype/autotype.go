package autotype

import (
	"log/slog"
)

// Paste inserts text into the currently focused input field.
// pasteDelayMs is the wait after writing the clipboard and before simulating paste.
func Paste(text string, pasteDelayMs int, logger *slog.Logger) error {
	return pastePlatform(text, pasteDelayMs, logger)
}
