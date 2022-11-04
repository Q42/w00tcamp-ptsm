package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Waits until the parent process wants us to shut down
		handleSignals(logger)
		cancel()
	}()

	tlsConfig := getTLSConfig(logger)
	go startSmtpServers(ctx, logger, tlsConfig)
	go startImapServers(ctx, logger, tlsConfig)
	<-ctx.Done()
}

func handleSignals(log *zap.Logger) {
	// Wait for SIGINT, SIGQUIT, or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	sig := <-sigs

	log.With(zap.Stringer("signal", sig)).
		Info("shutting down in response to received signal")
}
