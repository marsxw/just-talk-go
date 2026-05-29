package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/c/just-talk-go/hotkey"
)

// Engine is the application coordinator.
//
// It manages:
//   - The hotkey registry (platform-specific provider)
//   - Plugin loading and lifecycle
//   - Graceful shutdown on OS signals
type Engine struct {
	registry *hotkey.Registry
	provider hotkey.Provider
	plugins  []Plugin
	logger   *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.Mutex
}

// New creates a new Engine with the given hotkey provider and logger.
func New(provider hotkey.Provider, logger *slog.Logger) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		provider: provider,
		registry: hotkey.NewRegistry(provider),
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Provider returns the underlying hotkey provider.
func (e *Engine) Provider() hotkey.Provider {
	return e.provider
}

// Registry returns the hotkey registry.
func (e *Engine) Registry() *hotkey.Registry {
	return e.registry
}

// Logger returns the engine's logger.
func (e *Engine) Logger() *slog.Logger {
	return e.logger
}

// LoadPlugin adds a plugin to the engine. Must be called before Start().
func (e *Engine) LoadPlugin(p Plugin) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pluginEnv := &pluginEnvAdapter{
		engine:  e,
		plugin:  p,
		handler: e.registry,
		logger:  e.logger.With("plugin", p.Name()),
	}

	if err := p.Init(pluginEnv); err != nil {
		return fmt.Errorf("plugin %s init: %w", p.Name(), err)
	}

	e.plugins = append(e.plugins, p)
	e.logger.Info("plugin loaded", "name", p.Name(), "version", p.Version())
	return nil
}

// Start begins the engine's event loop.
//
// It starts all plugins in their own goroutines, then starts the
// hotkey registry (which blocks). When the registry returns, the
// engine cancels all plugin contexts and waits for them to stop.
//
// If waitSignal is true, Start also listens for SIGINT/SIGTERM
// and calls Stop() when received.
func (e *Engine) Start(waitSignal bool) error {
	e.logger.Info("starting engine",
		"platform", e.provider.Info().Platform,
		"backend", e.provider.Info().Backend,
	)

	// Start plugins
	for _, p := range e.plugins {
		p := p // capture
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			if err := p.Start(e.ctx); err != nil && err != context.Canceled {
				e.logger.Error("plugin exited with error", "plugin", p.Name(), "error", err)
			}
		}()
	}

	// Signal handling
	if waitSignal {
		go e.handleSignals()
	}

	// Run the hotkey registry (blocks)
	err := e.registry.Start(e.ctx)

	// Shutdown
	e.logger.Info("engine stopping")
	e.cancel() // Cancel all plugin contexts

	// Stop all plugins
	for _, p := range e.plugins {
		if err := p.Stop(); err != nil {
			e.logger.Error("plugin stop error", "plugin", p.Name(), "error", err)
		}
	}

	e.wg.Wait()
	e.logger.Info("engine stopped")

	return err
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("engine stop requested")
	e.cancel()

	if err := e.registry.Stop(); err != nil {
		e.logger.Error("registry stop error", "error", err)
	}
}

// Context returns the engine's context (cancelled on shutdown).
func (e *Engine) Context() context.Context {
	return e.ctx
}

// Wait blocks until all plugins have exited.
func (e *Engine) Wait() {
	e.wg.Wait()
}

func (e *Engine) handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		e.logger.Info("received signal", "signal", sig)
		e.Stop()
	case <-e.ctx.Done():
		// Engine already stopping
	}
	signal.Stop(sigCh)
}

// ---- pluginEnvAdapter implements PluginEnv ----

type pluginEnvAdapter struct {
	engine  *Engine
	plugin  Plugin
	handler *hotkey.Registry
	logger  *slog.Logger
}

func (a *pluginEnvAdapter) RegisterHotkey(combo hotkey.Combo, handler func(hotkey.Event)) error {
	return a.handler.Register(combo, handler)
}

func (a *pluginEnvAdapter) UnregisterHotkey(combo hotkey.Combo) error {
	return a.handler.Unregister(combo)
}

func (a *pluginEnvAdapter) Logger() *slog.Logger {
	return a.logger
}

func (a *pluginEnvAdapter) Engine() *Engine {
	return a.engine
}
