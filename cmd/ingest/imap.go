package main

import (
	"context"
	"crypto/tls"
	"log"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"go.uber.org/zap"
)

const (
	ImapAddr = ":993"
)

func startImapServers(ctx context.Context, logger *zap.Logger, tlsConfig *tls.Config) {
	be := memory.New()
	// Create a new server
	s := server.New(be)
	s.Addr = ImapAddr // 143 is the insecure port
	s.TLSConfig = tlsConfig
	s.EnableAuth(sasl.OAuthBearer, func(conn server.Conn) sasl.Server {
		return sasl.NewOAuthBearerServer(func(opts sasl.OAuthBearerOptions) *sasl.OAuthBearerError {
			// TODO check this token!
			_ = opts.Token
			if strings.HasSuffix(opts.Username, ".q42.nl") {
				ctx := conn.Context()
				ctx.State = imap.AuthenticatedState
				var err error
				ctx.User, err = be.Login(conn.Info(), "username", "password")
				return &sasl.OAuthBearerError{Status: err.Error()}
			}
			return &sasl.OAuthBearerError{
				Status:  "invalid_request",
				Schemes: "bearer",
			}
		})
	})

	go func() {
		<-ctx.Done()
		s.Close()
	}()

	log.Println("Starting IMAP server at " + ImapAddr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
