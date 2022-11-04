package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment(zap.IncreaseLevel(zap.DebugLevel))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Waits until the parent process wants us to shut down
		handleSignals(logger)
		cancel()
	}()

	flagset.Parse(os.Args[1:])
	tlsConfig, err := startHttpServer(ctx, logger.Named("http"))
	if err != nil {
		log.Fatal(err)
	}
	go startSmtpServers(ctx, logger.Named("smtp"), tlsConfig)
	go startImapServers(ctx, logger.Named("imap"), tlsConfig)
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

