package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/microrun/microrun/userspace/runtimed/logging"
	"github.com/microrun/microrun/userspace/runtimed/network"
	"github.com/microrun/microrun/userspace/runtimed/runtime"
	"github.com/microrun/microrun/userspace/runtimed/state"
)

func main() {
	logger := logging.NewLogger("runtimed", logging.ComponentController)

	// Create store
	store := state.NewMemoryStore()

	// Create runtime
	rt := runtime.New(store)

	// Register generators
	ifaceType := &network.InterfaceGeneratorType{}
	if err := rt.RegisterGenerator(ifaceType); err != nil {
		logger.Fatal("Failed to register generator",
			zap.String("name", ifaceType.Name()),
			zap.Error(err))
	}

	// Create context for runtime
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start runtime
	if err := rt.Start(ctx); err != nil {
		logger.Fatal("Failed to start runtime",
			zap.Error(err))
	}

	logger.Info("Runtime started")

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down")

	// Give components time to shutdown gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := rt.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown",
			zap.Error(err))
	}
}
