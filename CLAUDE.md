# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
make build              # Build for current platform
make build-all          # Cross-compile for linux/darwin/windows (amd64, arm64)
make run                # Run on current platform (auto-detects X11/Wayland)
make run-mock           # Run with mock provider (no hardware needed, any platform)
make test               # Run all tests
make test-mock          # Run tests with mock build tag
```

`JUST_TALK_BACKEND` env var or `--backend` flag overrides auto-detection: `x11`, `wayland`, `darwin`, `windows`, `mock`.

## Architecture

```
cmd/just-talk/main.go   → entry point, creates Provider → Engine → loads Plugins → Engine.Start()
                              │                      │
hotkey/                  ◄────┘                      │
  Provider interface     ← platform-specific backends │
  Registry               ← fans out channel events to callbacks
  KeyStateTracker        ← low-level key state machine (modifier-only detection)
                              │
engine/                                             ◄─┘
  Engine                 ← lifecycle orchestrator, signal handling
  Plugin interface       ← Init(PluginEnv) → Start(ctx) → Stop()
```

### Core types (`hotkey/hotkey.go`, `hotkey/keycodes.go`)

- **`Combo`** = `{Mods Modifier, Key KeyCode}` — a hotkey combination. Modifier-only: `{ModCtrl, KeyNone}`. Function-key-only: `{ModNone, KeyF1}`. Combo: `{ModCtrl|ModShift, KeyF}`.
- **`Event`** = `{Combo, Type (KeyDown/KeyUp/KeyPress), Time}` — emitted when a hotkey fires.
- **`Modifier`** is a bitmask: `ModNone=0, ModCtrl=1, ModAlt=2, ModShift=4, ModSuper=8`.
- **`KeyCode`** is a unified uint32 across all platforms. Modifier virtual keys are 256-259. Function keys are 290-313. Each platform's provider maps its native keycodes to these.

### Provider interface (`hotkey/provider.go`)

`Register(combo) → (<-chan Event, error)` — each combo gets its own channel. Channels must be **buffered** (size 32) to avoid dropped events before the dispatch goroutine is scheduled.

`Start(ctx)` blocks until `Stop()` or context cancellation. Must be called after all `Register` calls.

`Info()` returns `ProviderInfo{Platform, Backend, Features}` — `Features` lists supported capabilities like `"modifier-only"`, `"keydown"`, `"keyup"`.

### Platform backends (build tags: `darwin`, `windows`, `linux`)

| Platform | Backend | API | Key constraints |
|----------|---------|-----|-----------------|
| macOS | CGEventTap → Carbon fallback | cgo + ApplicationServices/Carbon | CGEventTap needs Accessibility permission; Carbon is zero-permission but misses modifier-only |
| Windows | WH_KEYBOARD_LL | `golang.org/x/sys/windows` | No admin needed. `runtime.LockOSThread()` required for message pump |
| Linux X11 | XGrabKey | cgo + libX11 | No root needed. Must use `owner_events=False` + specific modifier masks |
| Linux Wayland | evdev (`/dev/input/event*`) | `golang.org/x/sys/unix` | Needs `input` group or root |

Linux auto-detection: checks `XDG_SESSION_TYPE` → wayland or x11. Overridable via `JUST_TALK_BACKEND`.

### Registry (`hotkey/registry.go`)

Manages multiple hotkeys on one Provider. `Register(combo, handler)` stores the handler and spawns a dispatch goroutine in `Start()` that reads from the per-combo channel and calls the handler.

**Critical ordering**: Plugin hotkey registration must happen in `Plugin.Init()` (called synchronously in `Engine.LoadPlugin()`), NOT in `Plugin.Start()` (which runs in a goroutine after `Registry.Start()`). Otherwise the registry's dispatch goroutines launch before handlers exist, and events are silently dropped.

### KeyStateTracker (`hotkey/tracker.go`)

Used by providers that see all raw key events (CGEventTap, WH_KEYBOARD_LL, evdev). Tracks pressed keys + active modifiers, detects modifier-only hotkeys by tracking whether a non-modifier key was pressed while the modifier was held. X11 provider does NOT use the tracker (it only sees grabbed keys via XGrabKey).

### Engine & Plugins (`engine/`)

`Engine.New(provider, logger)` → `LoadPlugin(p)` → `Start(waitSignal)` blocks until SIGINT/SIGTERM or `Stop()`.

`Plugin` interface: `Name()`, `Version()`, `Init(PluginEnv)`, `Start(ctx)`, `Stop()`.

`PluginEnv` gives plugins access to `RegisterHotkey(combo, handler)`, `UnregisterHotkey(combo)`, a logger, and the engine reference.

## X11 Implementation Details (lessons from debugging)

1. **`owner_events` must be `False`**: With `True`, `XGrabKey` fails with `BadAccess` if ANY other client has an active keyboard grab (window managers always do).
2. **Never use `AnyModifier`**: It conflicts with grabs held by the WM/DE. Always grab with specific modifier masks.
3. **The `0` mask is essential**: The ignored-mask iteration must include `0` as the base case (no CapsLock/NumLock/ScrollLock). Without it, the key works only when a lock key is active.
4. **`BadAccess` on CapsLock/NumLock variants is expected**: Some of the 8 mask combinations will fail because another client holds them. The base `mod=0` case almost always succeeds. Errors are silently swallowed by an X error handler installed at startup.
5. **`XGrabKey` events are received via the normal `XNextEvent` queue** — no special handling needed. Auto-repeat is filtered by tracking pressed keycodes.

## Dependencies

- `golang.org/x/sys` — Windows syscalls, Linux evdev ioctl
- `github.com/godbus/dbus/v5` — planned for Wayland XDG Desktop Portal (not yet wired in)
- No third-party hotkey libraries (robotgo, golang.design/x/hotkey). Direct platform API calls for full control over modifier-only and push-to-talk scenarios.
