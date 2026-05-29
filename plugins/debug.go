// Package plugins provides built-in plugins for just-talk-go.
package plugins

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/c/just-talk-go/engine"
	"github.com/c/just-talk-go/hotkey"
)

// DebugPlugin is a simple plugin that registers a set of hotkeys and prints
// events to the log. It is useful for verifying the hotkey system works.
type DebugPlugin struct {
	env      engine.PluginEnv
	logger   *slog.Logger
	stopChan chan struct{}
}

// NewDebugPlugin creates a DebugPlugin with a default set of test hotkeys.
// The testKeys parameter can be used to override the default hotkeys.
func NewDebugPlugin(testKeys []hotkey.Combo) *DebugPlugin {
	if len(testKeys) == 0 {
		// Default test keys covering all three scenarios:
		// 1. Modifier-only: Ctrl alone, Shift alone
		// 2. Function-key-only: F1 alone
		// 3. Combination: Ctrl+Shift+F
		testKeys = []hotkey.Combo{
			{Mods: hotkey.ModCtrl, Key: hotkey.KeyNone},  // Ctrl alone
			{Mods: hotkey.ModNone, Key: hotkey.KeyF1},    // F1 alone
			{Mods: hotkey.ModCtrl | hotkey.ModShift, Key: hotkey.KeyF}, // Ctrl+Shift+F
		}
	}

	return &DebugPlugin{
		stopChan: make(chan struct{}),
	}
}

func (p *DebugPlugin) Name() string    { return "debug" }
func (p *DebugPlugin) Version() string { return "0.1.0" }

func (p *DebugPlugin) Init(env engine.PluginEnv) error {
	p.env = env
	p.logger = env.Logger()

	// Register hotkeys in Init, NOT Start.
	// Init is called synchronously before the registry starts its dispatch loop.
	testKeys := []hotkey.Combo{
		// Modifier-only
		{Mods: hotkey.ModCtrl, Key: hotkey.KeyNone},                       // Ctrl alone
		{Mods: hotkey.ModCtrl | hotkey.ModSuper, Key: hotkey.KeyNone},     // Ctrl+Super (multi-mod)
		{Mods: hotkey.ModSuper | hotkey.ModAlt, Key: hotkey.KeyNone},      // Super+Alt (multi-mod)
		// Function-key-only
		{Mods: hotkey.ModNone, Key: hotkey.KeyF1},                         // F1 alone
		// Combinations
		{Mods: hotkey.ModCtrl | hotkey.ModShift, Key: hotkey.KeyF},        // Ctrl+Shift+F
		{Mods: hotkey.ModCtrl | hotkey.ModSuper | hotkey.ModAlt, Key: hotkey.KeyT}, // Ctrl+Super+Alt+T
	}

	for _, combo := range testKeys {
		combo := combo // capture
		if err := p.env.RegisterHotkey(combo, func(evt hotkey.Event) {
			fmt.Printf("🔔 HOTKEY: %-40s | Type: %-8s | Time: %s\n",
				evt.Combo, evt.Type, evt.Time.Format("15:04:05.000"))
		}); err != nil {
			p.logger.Warn("failed to register test hotkey", "combo", combo, "error", err)
		}
	}

	return nil
}

func (p *DebugPlugin) Start(ctx context.Context) error {
	p.logger.Info("debug plugin started — listening for hotkeys")
	<-ctx.Done()
	return ctx.Err()
}

func (p *DebugPlugin) Stop() error {
	p.logger.Info("debug plugin stopped")
	return nil
}
