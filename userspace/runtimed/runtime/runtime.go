package runtime

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/microrun/microrun/userspace/runtimed/logging"
	"github.com/microrun/microrun/userspace/runtimed/state"
)

// GeneratorType defines a type of generator and how to create instances of it
type GeneratorType interface {
	// Name returns the unique name for this type of generator
	Name() string
	// ManagedKinds returns the resource kinds this generator manages
	ManagedKinds() []string
	// New creates a new generator instance with runtime-provided dependencies
	New(ctx GeneratorContext) (Generator, error)
}

// Generator is a runtime component that manages resources
type Generator interface {
	// Run starts the generator and blocks until context is cancelled
	Run(ctx context.Context) error
}

// GeneratorContext provides runtime-managed dependencies to generators
type GeneratorContext struct {
	Store  state.Store
	Logger *logging.Logger
}

// Runtime manages the lifecycle of all components
type Runtime struct {
	logger     *logging.Logger
	store      state.Store
	generators map[string]Generator
	wg         sync.WaitGroup
}

// New creates a new runtime instance
func New(store state.Store) *Runtime {
	return &Runtime{
		logger:     logging.NewLogger("runtime", logging.ComponentController),
		store:      store,
		generators: make(map[string]Generator),
	}
}

// Store returns the runtime's store
func (r *Runtime) Store() state.Store {
	return r.store
}

// RegisterGenerator adds a generator to the runtime
func (r *Runtime) RegisterGenerator(genType GeneratorType) error {
	name := genType.Name()
	if _, exists := r.generators[name]; exists {
		r.logger.Error("Generator already registered", 
			zap.String("name", name))
		return fmt.Errorf("generator %s already registered", name)
	}

	// Create restricted store for generator
	kinds := genType.ManagedKinds()
	restrictedStore := state.NewTypeRestrictedStore(r.store, kinds)
	restrictedStore = state.NewOwnershipStore(restrictedStore, name)

	// Create generator with runtime-provided dependencies
	gen, err := genType.New(GeneratorContext{
		Store:  restrictedStore,
		Logger: logging.NewLogger(name, logging.ComponentGenerator),
	})
	if err != nil {
		return fmt.Errorf("failed to create generator %s: %w", name, err)
	}

	r.generators[name] = gen
	r.logger.Info("Registered generator", 
		zap.String("name", name),
		zap.Strings("managed_kinds", kinds))
	return nil
}

// Start initializes and runs all components
func (r *Runtime) Start(ctx context.Context) error {
	r.logger.Info("Starting runtime")

	// Start all generators
	for name, gen := range r.generators {
		r.wg.Add(1)
		go func(name string, gen Generator) {
			defer r.wg.Done()
			if err := gen.Run(ctx); err != nil {
				r.logger.Error("Generator failed",
					zap.String("name", name),
					zap.Error(err))
			}
		}(name, gen)
	}

	// Wait for all generators to finish
	r.wg.Wait()
	return nil
}

// Stop gracefully shuts down all components
func (r *Runtime) Stop(ctx context.Context) error {
	r.logger.Info("Stopping runtime")
	
	// Wait for all components to stop with context timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("Runtime stopped gracefully")
		return nil
	case <-ctx.Done():
		r.logger.Warn("Runtime stop timed out")
		return ctx.Err()
	}
}
