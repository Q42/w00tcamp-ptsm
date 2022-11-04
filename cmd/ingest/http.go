package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/cors"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
)

func startHttpServer(ctx context.Context, logger *zap.Logger) (tlsConfig *tls.Config, err error) {
	r, err := NewProvisionServer()
	if err != nil {
		return nil, err
	}

	s := http.Server{Addr: ":443", Handler: cors.Default().Handler(r)}
	go func() {
		<-ctx.Done()
		s.Shutdown(context.Background())
	}()

	// Automatically provision HTTPS using LetsEncrypt
	dataDir := "/etc/autocert/live/"
	if err = os.MkdirAll(dataDir, 0644); err != nil {
		return nil, err
	}
	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(ctx context.Context, host string) error {
			if host == *hostName {
				return nil
			}
			return fmt.Errorf("acme/autocert: only %s host is allowed", *hostName)
		},
		Cache: autocert.DirCache(dataDir),
	}
	s.TLSConfig = m.TLSConfig()
	r.TLSConfig = m.TLSConfig()

	// Start the server
	go func() {
		logger.Info("Starting HTTPS server on :443")
		if err = s.ListenAndServeTLS("", ""); err != nil {
			logger.Fatal(err.Error(), zap.Error(err))
		}
	}()

	return &tls.Config{GetCertificate: m.GetCertificate}, err

}
