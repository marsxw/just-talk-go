// just-talk is a cross-platform global hotkey daemon.
//
// It listens for global hotkeys and dispatches events to plugins.
// Future plugins include voice input, text expansion, and more.
//
// Usage:
//
//	just-talk [flags]
//
// Flags:
//
//	--backend string   Force a specific backend (x11, wayland, darwin, windows, mock)
//	--debug            Enable the debug plugin (prints hotkey events to stdout)
//	--verbose          Enable verbose logging
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/c/just-talk-go/engine"
	"github.com/c/just-talk-go/hotkey"
	"github.com/c/just-talk-go/plugins"
)

func main() {
	backend := flag.String("backend", "", "force backend: x11, wayland, darwin, windows, mock")
	debug := flag.Bool("debug", true, "enable debug plugin")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()

	// Configure logging
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	// Allow backend override via environment if not set via flag
	if *backend == "" {
		*backend = os.Getenv("JUST_TALK_BACKEND")
	}
	if *backend != "" {
		os.Setenv("JUST_TALK_BACKEND", *backend)
	}

	// Create provider
	provider, err := createProvider(*backend)
	if err != nil {
		logger.Error("failed to create provider", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nTroubleshooting:\n")
		fmt.Fprintf(os.Stderr, "  X11:      Ensure $DISPLAY is set\n")
		fmt.Fprintf(os.Stderr, "  Wayland:  Add user to 'input' group: sudo usermod -aG input $USER\n")
		fmt.Fprintf(os.Stderr, "  macOS:    Grant Accessibility permission in System Settings\n")
		fmt.Fprintf(os.Stderr, "  Test:     Use --backend mock\n")
		os.Exit(1)
	}

	info := provider.Info()
	logger.Info("provider created",
		"platform", info.Platform,
		"backend", info.Backend,
		"features", info.Features,
	)

	// Create engine
	eng := engine.New(provider, logger)

	// Load debug plugin
	if *debug {
		dp := plugins.NewDebugPlugin(nil)
		if err := eng.LoadPlugin(dp); err != nil {
			logger.Error("failed to load debug plugin", "error", err)
			os.Exit(1)
		}
	}

	// Start and run until signal
	logger.Info("just-talk started — press hotkeys to see events, Ctrl+C to quit")
	if err := eng.Start(true); err != nil && err != context.Canceled {
		logger.Error("engine exited with error", "error", err)
		os.Exit(1)
	}
}

func createProvider(backend string) (hotkey.Provider, error) {
	if backend == "mock" {
		return hotkey.NewMockProvider(), nil
	}
	return hotkey.NewProvider()
}
