package main

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/emersion/go-msgauth/dkim"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment(zap.IncreaseLevel(zap.DebugLevel))
	zap.ReplaceGlobals(logger)

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
	if err != nil {
		log.Fatal(err)
	}

	go startSmtpServers(ctx, logger.Named("smtp"), tlsConfig, dkimSigner(tlsConfig, logger))
	go startImapServers(ctx, logger.Named("imap"), tlsConfig)
	<-ctx.Done()
}

func dkimOpts(c *tls.Config, logger *zap.Logger) (*dkim.SignOptions, error) {
	cert, err := c.GetCertificate(&tls.ClientHelloInfo{ServerName: *hostName})
	if err != nil {
		return nil, err
	}
	if dkimSelector == nil || *dkimSelector == "" {
		sig := hex.EncodeToString(cert.Leaf.Signature)[0:10]
		dkimSelector = &sig
		logger.Sugar().Infof("Using generated DKIM selector %q for domain %q", *dkimSelector, *domain)
	}
	return &dkim.SignOptions{
		Signer:   asSigner(cert.PrivateKey),
		Domain:   *domain,
		Selector: *dkimSelector,
	}, nil
}

func dkimSigner(c *tls.Config, logger *zap.Logger) func() (*dkim.Signer, error) {
	return func() (s *dkim.Signer, err error) {
		var opts *dkim.SignOptions
		if opts, err = dkimOpts(c, logger); err != nil {
			return nil, err
		}
		return dkim.NewSigner(opts)
	}
}

func handleSignals(log *zap.Logger) {
	// Wait for SIGINT, SIGQUIT, or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	sig := <-sigs

	log.With(zap.Stringer("signal", sig)).
		Info("shutting down in response to received signal")
}
