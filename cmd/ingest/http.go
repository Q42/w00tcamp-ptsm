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
	r, err := NewProvisionServer(logger)
	if err != nil {
		return nil, err
	}

	s := http.Server{Addr: ":443", Handler: cors.Default().Handler(r), ErrorLog: zap.NewStdLog(logger)}
	go func() {
		<-ctx.Done()
		s.Shutdown(context.Background())
	}()

	c, err := makeTLSConfig(logger)
	s.TLSConfig = c
	r.TLSConfig = c
	if err != nil {
		return nil, err
	}

	// Start the server
	go func() {
		logger.Info("Starting HTTPS server on :443")
		if err = s.ListenAndServeTLS("", ""); err != nil {
			logger.Fatal(err.Error(), zap.Error(err))
		}
	}()

	return &tls.Config{GetCertificate: c.GetCertificate}, err

}

func makeTLSConfig(logger *zap.Logger) (c *tls.Config, err error) {
	// Load TLS certs from fixed files
	if *localCert != "" && *localKey != "" {
		logger.Info("Using configured certificate material")
		c = &tls.Config{Certificates: []tls.Certificate{{}}}
		c.Certificates[0], err = tls.LoadX509KeyPair(*localCert, *localKey)
		return
	}

	// Automatically provision HTTPS using LetsEncrypt
	logger.Info("Using autocert certificate material")
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
	return m.TLSConfig(), nil
}
