package hotkey

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockProvider is a mock hotkey provider for testing.
// It allows programmatic injection of hotkey events.
type MockProvider struct {
	mu       sync.Mutex
	channels map[Combo]chan<- Event
	stopped  bool
}

// NewMockProvider creates a new MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		channels: make(map[Combo]chan<- Event),
	}
}

// Register registers a hotkey combo.
func (m *MockProvider) Register(combo Combo) (<-chan Event, error) {
	return m.RegisterWithOptions(combo, RegisterOptions{})
}

func (m *MockProvider) RegisterWithOptions(combo Combo, opts RegisterOptions) (<-chan Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return nil, fmt.Errorf("provider is stopped")
	}

	if _, exists := m.channels[combo]; exists {
		return nil, fmt.Errorf("hotkey %s is already registered", combo)
	}

	ch := make(chan Event, 16) // buffered to avoid blocking tests
	m.channels[combo] = ch
	return ch, nil
}

// Unregister removes a hotkey combo.
func (m *MockProvider) Unregister(combo Combo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, exists := m.channels[combo]
	if !exists {
		return fmt.Errorf("hotkey %s is not registered", combo)
	}

	close(ch)
	delete(m.channels, combo)
	return nil
}

// Start is a no-op for the mock provider.
func (m *MockProvider) Start(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// Stop marks the provider as stopped.
func (m *MockProvider) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true

	// Close all channels
	for _, ch := range m.channels {
		close(ch)
	}
	m.channels = make(map[Combo]chan<- Event)
	return nil
}

// Info returns mock provider metadata.
func (m *MockProvider) Info() ProviderInfo {
	return ProviderInfo{
		Platform: "mock",
		Backend:  "mock",
		Features: []string{
			FeatureKeyDown,
			FeatureKeyUp,
			FeatureKeyPress,
			FeatureModifierOnly,
			FeatureFunctionKey,
			FeatureCombo,
		},
	}
}

// Simulate sends a hotkey event to all registered channels that match the combo.
// This is the primary testing API.
func (m *MockProvider) Simulate(combo Combo, eventType EventType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	evt := Event{
		Combo: combo,
		Type:  eventType,
		Time:  time.Now(),
	}

	if ch, ok := m.channels[combo]; ok {
		select {
		case ch <- evt:
		default:
			// Channel full, drop event (test should use buffered channels)
		}
	}
}

// SimulateKeyDown is a convenience method for KeyDown events.
func (m *MockProvider) SimulateKeyDown(combo Combo) {
	m.Simulate(combo, KeyDown)
}

// SimulateKeyUp is a convenience method for KeyUp events.
func (m *MockProvider) SimulateKeyUp(combo Combo) {
	m.Simulate(combo, KeyUp)
}

// SimulateKeyPress is a convenience method for KeyPress events.
func (m *MockProvider) SimulateKeyPress(combo Combo) {
	m.Simulate(combo, KeyDown)
	m.Simulate(combo, KeyUp)
	m.Simulate(combo, KeyPress)
}
