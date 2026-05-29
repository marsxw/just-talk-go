//go:build darwin

package hotkey

// #cgo LDFLAGS: -framework Carbon -framework ApplicationServices
//
// #include <ApplicationServices/ApplicationServices.h>
// #include <Carbon/Carbon.h>
// #include <pthread.h>
// #include <unistd.h>
// #include <string.h>
// #include <stdlib.h>
//
// // ---- Pipe-based event bridge (shared by CGEventTap and Carbon) ----
// static int bridge_fd = -1;
// static pthread_mutex_t bridge_mutex = PTHREAD_MUTEX_INITIALIZER;
//
// typedef enum {
// 	BRIDGE_KEY_DOWN       = 0,
// 	BRIDGE_KEY_UP         = 1,
// 	BRIDGE_FLAGS_CHANGED  = 2,
// 	BRIDGE_CARBON_HOTKEY  = 3,
// } bridge_event_type_t;
//
// typedef struct {
// 	uint16_t keycode;     // macOS keycode for keyboard events, 0 for carbon
// 	uint64_t flags;       // CGEventFlags for keyboard events
// 	uint8_t  event_type;  // bridge_event_type_t
// 	uint32_t carbon_id;   // hotkey ID for carbon events
// 	int64_t  time_ms;
// } bridge_event_t;
//
// static void bridge_init(int fd) {
// 	bridge_fd = fd;
// }
//
// static void bridge_send(bridge_event_t *evt) {
// 	if (bridge_fd < 0) return;
// 	pthread_mutex_lock(&bridge_mutex);
// 	ssize_t n = write(bridge_fd, evt, sizeof(*evt));
// 	(void)n;
// 	pthread_mutex_unlock(&bridge_mutex);
// }
//
// // ---- CGEventTap callback ----
// static CGEventRef cg_event_cb(CGEventTapProxy proxy, CGEventType type,
// 	CGEventRef event, void *refcon) {
// 	CGKeyCode kc = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
// 	CGEventFlags fl = CGEventGetFlags(event);
// 	bridge_event_t evt;
// 	memset(&evt, 0, sizeof(evt));
// 	evt.keycode  = (uint16_t)kc;
// 	evt.flags    = (uint64_t)fl;
// 	evt.time_ms  = (int64_t)(CFAbsoluteTimeGetCurrent() * 1000.0);
//
// 	switch (type) {
// 	case kCGEventKeyDown:       evt.event_type = BRIDGE_KEY_DOWN;       break;
// 	case kCGEventKeyUp:         evt.event_type = BRIDGE_KEY_UP;         break;
// 	case kCGEventFlagsChanged:  evt.event_type = BRIDGE_FLAGS_CHANGED;   break;
// 	default: return event;
// 	}
// 	bridge_send(&evt);
// 	return event;
// }
//
// static CFMachPortRef darwin_create_tap(void) {
// 	CGEventMask mask = CGEventMaskBit(kCGEventKeyDown)
// 		| CGEventMaskBit(kCGEventKeyUp)
// 		| CGEventMaskBit(kCGEventFlagsChanged);
// 	return CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap,
// 		kCGEventTapOptionDefault, mask, cg_event_cb, NULL);
// }
//
// // ---- Carbon hotkey support ----
// #define MAX_CARBON_HOTKEYS 128
// static EventHotKeyRef carbon_refs[MAX_CARBON_HOTKEYS];
// static int carbon_count = 0;
//
// static OSStatus carbon_hotkey_cb(EventHandlerCallRef caller, EventRef event, void *userData) {
// 	EventHotKeyID hotkeyID;
// 	OSStatus err = GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID,
// 		NULL, sizeof(hotkeyID), NULL, &hotkeyID);
// 	if (err == noErr) {
// 		bridge_event_t evt;
// 		memset(&evt, 0, sizeof(evt));
// 		evt.event_type = BRIDGE_CARBON_HOTKEY;
// 		evt.carbon_id  = hotkeyID.id;
// 		evt.time_ms    = (int64_t)(CFAbsoluteTimeGetCurrent() * 1000.0);
// 		bridge_send(&evt);
// 	}
// 	return noErr;
// }
//
// static int darwin_register_carbon(uint32_t keycode, uint32_t mods, uint32_t id) {
// 	if (carbon_count >= MAX_CARBON_HOTKEYS) return -1;
// 	EventHotKeyRef ref = NULL;
// 	EventHotKeyID hid = {'j', id};
// 	OSStatus err = RegisterEventHotKey(keycode, mods, hid,
// 		GetApplicationEventTarget(), 0, &ref);
// 	if (err != noErr) return -1;
// 	carbon_refs[carbon_count++] = ref;
// 	return carbon_count - 1;
// }
//
// static void darwin_unregister_carbon(int idx) {
// 	if (idx < 0 || idx >= carbon_count) return;
// 	UnregisterEventHotKey(carbon_refs[idx]);
// 	carbon_refs[idx] = NULL;
// }
//
// static OSStatus darwin_install_carbon_handler(void) {
// 	EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
// 	return InstallEventHandler(GetApplicationEventTarget(),
// 		NewEventHandlerUPP(carbon_hotkey_cb), 1, &spec, NULL, NULL);
// }
//
// // ---- Accessibility ----
// static bool darwin_check_accessibility(void) {
// 	return AXIsProcessTrusted();
// }
//
// static void darwin_request_accessibility(void) {
// 	CFStringRef keys[] = {kAXTrustedCheckOptionPrompt};
// 	CFBooleanRef vals[] = {kCFBooleanTrue};
// 	CFDictionaryRef opts = CFDictionaryCreate(NULL,
// 		(const void **)keys, (const void **)vals, 1,
// 		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
// 	AXIsProcessTrustedWithOptions(opts);
// 	if (opts) CFRelease(opts);
// }
import "C"

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// macOS keycode → unified KeyCode (subset).
var darwinKeyToUnified = map[uint16]KeyCode{
	0x00: KeyA, 0x0B: KeyB, 0x08: KeyC, 0x02: KeyD,
	0x0E: KeyE, 0x03: KeyF, 0x05: KeyG, 0x04: KeyH,
	0x22: KeyI, 0x26: KeyJ, 0x28: KeyK, 0x25: KeyL,
	0x2E: KeyM, 0x2D: KeyN, 0x1F: KeyO, 0x23: KeyP,
	0x0C: KeyQ, 0x0F: KeyR, 0x01: KeyS, 0x11: KeyT,
	0x20: KeyU, 0x09: KeyV, 0x0D: KeyW, 0x07: KeyX,
	0x10: KeyY, 0x06: KeyZ,

	0x1D: Key0, 0x12: Key1, 0x13: Key2, 0x14: Key3, 0x15: Key4,
	0x17: Key5, 0x16: Key6, 0x1A: Key7, 0x1C: Key8, 0x19: Key9,

	0x3B:  KeyCtrl,  0x3A: KeyAlt, 0x38: KeyShift, 0x37: KeySuper,
	0x3E:  KeyCtrl,  0x3D: KeyAlt, 0x3C: KeyShift, 0x36: KeySuper,

	0x7A: KeyF1, 0x78: KeyF2, 0x63: KeyF3, 0x76: KeyF4,
	0x60: KeyF5, 0x61: KeyF6, 0x62: KeyF7, 0x64: KeyF8,
	0x65: KeyF9, 0x6D: KeyF10, 0x67: KeyF11, 0x6F: KeyF12,
	0x69: KeyF13, 0x6B: KeyF14, 0x71: KeyF15, 0x6A: KeyF16,
	0x40: KeyF17, 0x4F: KeyF18, 0x50: KeyF19, 0x5A: KeyF20,

	0x31: KeySpace, 0x30: KeyTab, 0x24: KeyEnter, 0x35: KeyEscape,
	0x33: KeyBackspace, 0x39: KeyCapsLock,
	0x7E: KeyArrowUp, 0x7D: KeyArrowDown, 0x7B: KeyArrowLeft, 0x7C: KeyArrowRight,
	0x73: KeyHome, 0x77: KeyEnd, 0x74: KeyPageUp, 0x79: KeyPageDown,
	0x72: KeyInsert, 0x75: KeyDelete,

	0x32: KeyBacktick, 0x1B: KeyMinus, 0x18: KeyEqual,
	0x21: KeyLeftBracket, 0x1E: KeyRightBracket,
	0x2A: KeyBackslash, 0x29: KeySemicolon, 0x27: KeyQuote,
	0x2B: KeyComma, 0x2F: KeyPeriod, 0x2C: KeySlash,
}

func darwinFlagsToMods(flags C.uint64_t) Modifier {
	var m Modifier
	if flags&C.kCGEventFlagMaskControl != 0 {
		m |= ModCtrl
	}
	if flags&C.kCGEventFlagMaskAlternate != 0 {
		m |= ModAlt
	}
	if flags&C.kCGEventFlagMaskShift != 0 {
		m |= ModShift
	}
	if flags&C.kCGEventFlagMaskCommand != 0 {
		m |= ModSuper
	}
	return m
}

type darwinProvider struct {
	mu       sync.Mutex
	channels map[Combo]chan<- Event
	tracker  *KeyStateTracker
	stopped  bool

	// Pipe for C→Go event bridge
	pipeFd int
	evtBuf []byte

	// CGEventTap
	tap       C.CFMachPortRef
	tapSource C.CFRunLoopSourceRef

	// Carbon fallback
	carbonIDs map[uint32]Combo // hotkey ID → combo mapping
	useCarbon bool

	logger *slog.Logger
}

// export darwinNewProvider
func NewProvider() (Provider, error) {
	logger := slog.Default().With("platform", "darwin")

	p := &darwinProvider{
		channels:  make(map[Combo]chan<- Event),
		tracker:   NewKeyStateTracker(),
		carbonIDs: make(map[uint32]Combo),
		evtBuf:    make([]byte, C.sizeof_bridge_event_t),
		logger:    logger,
	}

	if bool(C.darwin_check_accessibility()) {
		logger.Info("accessibility permission granted, using CGEventTap")
	} else {
		logger.Warn("no accessibility permission, falling back to Carbon")
		C.darwin_request_accessibility()
		p.useCarbon = true
	}

	return p, nil
}

func (p *darwinProvider) Register(combo Combo) (<-chan Event, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return nil, fmt.Errorf("provider is stopped")
	}
	if _, exists := p.channels[combo]; exists {
		return nil, fmt.Errorf("hotkey %s already registered", combo)
	}

	ch := make(chan Event, 32)
	p.channels[combo] = ch

	if p.useCarbon {
		if err := p.registerCarbon(combo); err != nil {
			delete(p.channels, combo)
			return nil, err
		}
	} else {
		p.tracker.Watch(combo, ch)
	}

	return ch, nil
}

func (p *darwinProvider) registerCarbon(combo Combo) error {
	if combo.IsModifierOnly() {
		p.logger.Warn("carbon does not support modifier-only", "combo", combo)
		return nil // accepted, won't fire
	}

	kvk := unifiedToDarwinKeyCode(combo.Key)
	if kvk == 0xFFFF {
		return fmt.Errorf("unsupported key: %s", combo.Key)
	}

	mods := modsToCarbonMods(combo.Mods)
	id := uint32(len(p.carbonIDs) + 1)
	p.carbonIDs[id] = combo

	rc := C.darwin_register_carbon(C.uint32_t(kvk), C.uint32_t(mods), C.uint32_t(id))
	if rc < 0 {
		delete(p.carbonIDs, id)
		return fmt.Errorf("RegisterEventHotKey failed for %s", combo)
	}

	return nil
}

func (p *darwinProvider) Unregister(combo Combo) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch, exists := p.channels[combo]
	if !exists {
		return fmt.Errorf("hotkey %s not registered", combo)
	}

	if p.useCarbon {
		for id, c := range p.carbonIDs {
			if c == combo {
				C.darwin_unregister_carbon(C.int(id - 1))
				delete(p.carbonIDs, id)
				break
			}
		}
	} else {
		p.tracker.Unwatch(combo)
	}

	close(ch)
	delete(p.channels, combo)
	return nil
}

func (p *darwinProvider) Start(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Create pipe for C→Go communication
	fds := make([]int, 2)
	if err := syscall.Pipe(fds); err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	p.pipeFd = fds[0]
	C.bridge_init(C.int(fds[1]))

	// Start Go-side reader
	go p.readPipe(ctx)

	if p.useCarbon {
		return p.runCarbonLoop(ctx)
	}
	return p.runCGEventTap(ctx)
}

func (p *darwinProvider) runCGEventTap(ctx context.Context) error {
	p.tap = C.darwin_create_tap()
	if p.tap == nil {
		return fmt.Errorf("CGEventTapCreate failed — check accessibility permission")
	}

	p.tapSource = C.CFMachPortCreateRunLoopSource(
		C.kCFAllocatorDefault, p.tap, 0,
	)
	if p.tapSource == nil {
		C.CFMachPortInvalidate(p.tap)
		C.CFRelease(C.CFTypeRef(p.tap))
		return fmt.Errorf("CFMachPortCreateRunLoopSource failed")
	}

	C.CFRunLoopAddSource(C.CFRunLoopGetCurrent(), p.tapSource, C.kCFRunLoopCommonModes)
	C.CGEventTapEnable(p.tap, C.true)

	p.logger.Info("CGEventTap running")

	// Stop run loop when context is cancelled
	go func() {
		<-ctx.Done()
		C.CFRunLoopStop(C.CFRunLoopGetCurrent())
	}()

	C.CFRunLoopRun()

	// Cleanup
	C.CGEventTapEnable(p.tap, C.false)
	C.CFRunLoopRemoveSource(C.CFRunLoopGetCurrent(), p.tapSource, C.kCFRunLoopCommonModes)
	C.CFRelease(C.CFTypeRef(p.tapSource))
	C.CFMachPortInvalidate(p.tap)
	C.CFRelease(C.CFTypeRef(p.tap))
	syscall.Close(C.int(p.pipeFd))

	return ctx.Err()
}

func (p *darwinProvider) runCarbonLoop(ctx context.Context) error {
	if err := C.darwin_install_carbon_handler(); err != C.noErr {
		return fmt.Errorf("InstallEventHandler failed: %d", int(err))
	}

	p.logger.Info("Carbon event loop running")

	go func() {
		<-ctx.Done()
		C.CFRunLoopStop(C.CFRunLoopGetCurrent())
	}()

	C.CFRunLoopRun()

	syscall.Close(C.int(p.pipeFd))
	return ctx.Err()
}

func (p *darwinProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return nil
	}
	p.stopped = true

	// Close all channels
	for c, ch := range p.channels {
		close(ch)
		delete(p.channels, c)
		if !p.useCarbon {
			p.tracker.Unwatch(c)
		}
	}

	// Stop run loop
	C.CFRunLoopStop(C.CFRunLoopGetCurrent())

	return nil
}

func (p *darwinProvider) Info() ProviderInfo {
	backend := "CGEventTap"
	if p.useCarbon {
		backend = "Carbon"
	}
	features := []string{FeatureCombo, FeatureFunctionKey}
	if !p.useCarbon {
		features = append(features,
			FeatureKeyDown, FeatureKeyUp, FeatureKeyPress,
			FeatureModifierOnly, FeatureSuppressEvent,
		)
	}
	return ProviderInfo{
		Platform: "darwin",
		Backend:  backend,
		Features: features,
	}
}

// ---- Pipe reader (shared by CGEventTap and Carbon) ----

func (p *darwinProvider) readPipe(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := syscall.Read(p.pipeFd, p.evtBuf)
		if err != nil || n < int(C.sizeof_bridge_event_t) {
			if ctx.Err() != nil {
				return
			}
			// On EOF or error, back off briefly
			time.Sleep(100 * time.Millisecond)
			continue
		}

		evtType := p.evtBuf[18] // offset of event_type in bridge_event_t

		if evtType == C.BRIDGE_CARBON_HOTKEY {
			p.handleCarbonEvent()
		} else {
			p.handleCGEvent()
		}
	}
}

func (p *darwinProvider) handleCarbonEvent() {
	// Read carbon_id from the buffer
	carbonID := binary.LittleEndian.Uint32(p.evtBuf[19:23])

	p.mu.Lock()
	combo, ok := p.carbonIDs[carbonID]
	var ch chan<- Event
	if ok {
		// Re-lookup channel — use a two-step check
		ch = p.channels[combo]
	}
	p.mu.Unlock()

	if ch != nil {
		evt := Event{
			Combo: combo,
			Type:  KeyPress,
			Time:  time.Now(),
		}
		select {
		case ch <- evt:
		default:
		}
	}
}

func (p *darwinProvider) handleCGEvent() {
	keycode := binary.LittleEndian.Uint16(p.evtBuf[0:2])
	flags := binary.LittleEndian.Uint64(p.evtBuf[2:10])
	evtType := p.evtBuf[18]
	timeMs := int64(binary.LittleEndian.Uint64(p.evtBuf[24:32]))

	key := darwinKeyToUnified[keycode]
	if key == KeyNone {
		return
	}

	mods := darwinFlagsToMods(C.uint64_t(flags))
	now := time.UnixMilli(timeMs)

	var events []Event
	switch evtType {
	case C.BRIDGE_KEY_DOWN:
		events = p.tracker.KeyDown(key, now)
	case C.BRIDGE_KEY_UP:
		events = p.tracker.KeyUp(key, now)
	case C.BRIDGE_FLAGS_CHANGED:
		events = p.processFlagsChanged(key, mods, now)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range events {
		if ch, ok := p.channels[e.Combo]; ok {
			select {
			case ch <- e:
			default:
			}
		}
	}
}

func (p *darwinProvider) processFlagsChanged(key KeyCode, mods Modifier, now time.Time) []Event {
	// Modifier pressed if its bit is set
	var events []Event
	if mods&KeyCodeToModifier(key) != 0 {
		events = append(events, p.tracker.KeyDown(key, now)...)
	} else {
		events = append(events, p.tracker.KeyUp(key, now)...)
	}
	return events
}

// ---- Key code conversion ----

func unifiedToDarwinKeyCode(k KeyCode) uint16 {
	for dk, uk := range darwinKeyToUnified {
		if uk == k {
			return dk
		}
	}
	return 0xFFFF
}

func modsToCarbonMods(mods Modifier) C.uint32_t {
	var cm C.uint32_t
	if mods&ModCtrl != 0 {
		cm |= C.uint32_t(C.controlKey)
	}
	if mods&ModAlt != 0 {
		cm |= C.uint32_t(C.optionKey)
	}
	if mods&ModShift != 0 {
		cm |= C.uint32_t(C.shiftKey)
	}
	if mods&ModSuper != 0 {
		cm |= C.uint32_t(C.cmdKey)
	}
	return cm
}
