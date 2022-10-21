package main

import (
	"bytes"
	"crypto/tls"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chrj/smtpd"
	"github.com/xtgo/uuid"
	"go.uber.org/zap"
)

type protoAddr struct {
	protocol string
	address  string
}

type wrap struct {
	logger *zap.Logger
}

func main() {
	logger, _ := zap.NewProduction()
	var servers []*smtpd.Server
	for _, listen := range []protoAddr{{"", ":25"}, {"starttls", ":587"}, {"tls", ":465"}} {
		var err error
		var lsnr net.Listener

		w := wrap{logger.With(zap.String("protocol", listen.protocol))}
		server := &smtpd.Server{
			Hostname:          *hostName,
			WelcomeMessage:    *welcomeMsg,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			DataTimeout:       dataTimeout,
			MaxConnections:    *maxConnections,
			MaxMessageSize:    *maxMessageSize,
			MaxRecipients:     *maxRecipients,
			ConnectionChecker: w.connectionChecker,
			SenderChecker:     w.senderChecker,
			RecipientChecker:  w.recipientChecker,
			Handler:           w.mailHandler,
		}

		switch listen.protocol {
		case "":
			logger.Info("listening on address")
			lsnr, err = net.Listen("tcp", listen.address)

		case "starttls":
			server.TLSConfig = getTLSConfig(logger)
			server.ForceTLS = *localForceTLS

			logger.Info("listening on address (STARTTLS)")
			lsnr, err = net.Listen("tcp", listen.address)

		case "tls":
			server.TLSConfig = getTLSConfig(logger)

			logger.Info("listening on address (TLS)")
			lsnr, err = tls.Listen("tcp", listen.address, server.TLSConfig)

		default:
			logger.
				With(zap.String("protocol", listen.protocol)).
				Fatal("unknown protocol in listen address")
		}

		if err != nil {
			logger.With(zap.Error(err)).Fatal("error starting listener")
		}
		servers = append(servers, server)
		go func() {
			server.Serve(lsnr)
		}()
	}

	// Waits until the parent process wants us to shut down
	handleSignals(logger)

	// First close the listeners
	for _, server := range servers {
		logger := logger.With(zap.Any("address", server.Address()))
		logger.Debug("Shutting down server")
		err := server.Shutdown(false)
		if err != nil {
			logger.With(zap.Error(err)).
				Warn("Shutdown failed")
		}
	}

	// Then wait for the clients to exit
	for _, server := range servers {
		logger := logger.With(zap.Any("address", server.Address()))
		logger.Debug("Waiting for server")
		err := server.Wait()
		if err != nil {
			logger.With(zap.Error(err)).
				Warn("Wait failed")
		}
	}

	logger.Debug("done")
}

func (w wrap) connectionChecker(peer smtpd.Peer) error {
	// we listen openly on the internet, so each connection is OK
	return nil
}

func addrAllowed(addr string, allowedAddrs []string) bool {
	if allowedAddrs == nil {
		// If absent, all addresses are allowed
		return true
	}

	addr = strings.ToLower(addr)

	// Extract optional domain part
	domain := ""
	if idx := strings.LastIndex(addr, "@"); idx != -1 {
		domain = strings.ToLower(addr[idx+1:])
	}

	// Test each address from allowedUsers file
	for _, allowedAddr := range allowedAddrs {
		allowedAddr = strings.ToLower(allowedAddr)

		// Three cases for allowedAddr format:
		if idx := strings.Index(allowedAddr, "@"); idx == -1 {
			// 1. local address (no @) -- must match exactly
			if allowedAddr == addr {
				return true
			}
		} else {
			if idx != 0 {
				// 2. email address (user@domain.com) -- must match exactly
				if allowedAddr == addr {
					return true
				}
			} else {
				// 3. domain (@domain.com) -- must match addr domain
				allowedDomain := allowedAddr[idx+1:]
				if allowedDomain == domain {
					return true
				}
			}
		}
	}

	return false
}

func (w wrap) senderChecker(peer smtpd.Peer, addr string) error {
	if allowedSender == nil {
		// Any sender is permitted
		return nil
	}

	if allowedSender.MatchString(addr) {
		// Permitted by regex
		return nil
	}

	w.logger.
		With(zap.String("sender_address", addr), zap.Any("peer", peer.Addr)).
		Warn("sender address not allowed by allowed_sender pattern")
	return smtpd.Error{Code: 451, Message: "Bad sender address"}
}

func (w wrap) recipientChecker(peer smtpd.Peer, addr string) error {
	if allowedRecipients == nil {
		// Any recipient is permitted
		return nil
	}

	if allowedRecipients.MatchString(addr) {
		// Permitted by regex
		return nil
	}

	w.logger.
		With(zap.String("sender_address", addr), zap.Any("peer", peer.Addr)).
		Warn("recipient address not allowed by allowed_recipients pattern")
	return smtpd.Error{Code: 451, Message: "Bad recipient address"}
}

func (w wrap) mailHandler(peer smtpd.Peer, env smtpd.Envelope) error {
	peerIP := ""
	if addr, ok := peer.Addr.(*net.TCPAddr); ok {
		peerIP = addr.IP.String()
	}

	logger := w.logger.With(zap.String("from", env.Sender), zap.Strings("to", env.Recipients), zap.String("peer", peerIP), zap.String("uuid", generateUUID()))

	if *remotesStr == "" && *command == "" {
		logger.Warn("no remote_host or command set; discarding mail")
		return nil
	}

	env.AddReceivedLine(peer)

	if *command != "" {
		cmdLogger := logger.With(zap.String("command", *command))

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		cmd := exec.Command(*command)
		cmd.Stdin = bytes.NewReader(env.Data)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			cmdLogger.With(zap.Error(err)).Error(stderr.String())
			return smtpd.Error{Code: 554, Message: "External command failed"}
		}

		cmdLogger.Info("pipe command successful: " + stdout.String())
	}

	logger.With(zap.String("data", string(env.Data))).Info("TODO delivering mail from peer using smarthost")
	return nil
}

func generateUUID() string {
	uniqueID := uuid.NewRandom()
	return uniqueID.String()
}

func getTLSConfig(logger *zap.Logger) *tls.Config {
	// Ciphersuites as defined in stock Go but without 3DES and RC4
	// https://golang.org/src/crypto/tls/cipher_suites.go
	var tlsCipherSuites = []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256, // does not provide PFS
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384, // does not provide PFS
	}

	if *localCert == "" || *localKey == "" {
		logger.
			With(zap.String("cert_file", *localCert), zap.String("key_file", *localKey)).
			Fatal("TLS certificate/key file not defined in config")
	}

	cert, err := tls.LoadX509KeyPair(*localCert, *localKey)
	if err != nil {
		logger.With(zap.Error(err)).
			Fatal("cannot load X509 keypair")
	}

	return &tls.Config{
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
		CipherSuites:             tlsCipherSuites,
		Certificates:             []tls.Certificate{cert},
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
