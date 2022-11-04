package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bcampbell/tameimap/store"
	"github.com/chrj/smtpd"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/pkg/errors"
	"github.com/xtgo/uuid"
	"go.uber.org/zap"
)

type protoAddr struct {
	protocol string
	address  string
}

type wrap struct {
	logger    *zap.Logger
	getSigner func() (*dkim.Signer, error)
}

func startSmtpServers(ctx context.Context, logger *zap.Logger, tlsConfig *tls.Config, getSigner func() (*dkim.Signer, error)) {
	var servers []*smtpd.Server
	for _, listen := range []protoAddr{{"starttls", ":25"}, {"starttls", ":587"}, {"tls", ":465"}} {
		var err error
		var lsnr net.Listener

		w := wrap{logger.With(zap.String("protocol", listen.protocol)), getSigner}
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
			Authenticator:     w.authenticator,
			AllowAnonymous:    true,
		}

		switch listen.protocol {
		case "":
			logger.Info("listening on address")
			lsnr, err = net.Listen("tcp", listen.address)

		case "starttls":
			server.TLSConfig = tlsConfig
			server.ForceTLS = *localForceTLS

			logger.Info("listening on address (STARTTLS)")
			lsnr, err = net.Listen("tcp", listen.address)

		case "tls":
			server.TLSConfig = tlsConfig

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

	// Wait until shutdown is requested
	<-ctx.Done()

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
	if peer.Username != "" {
		// TODO verify if the mail is FROM one of us or TO one of us
		_ = 0
	}

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

func (w wrap) authenticator(peer smtpd.Peer, username, password string) error {
	return FirestoreAuthenticator(context.Background(), w.logger, peer, username, password)
}

func (w wrap) mailHandler(peer smtpd.Peer, env smtpd.Envelope) (err error) {
	logger := w.logger.With(zap.String("from", env.Sender), zap.Strings("to", env.Recipients), zap.String("uuid", generateUUID()))
	defer func() {
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to handle mail").Error(), zap.Error(err))
			if !errors.Is(err, smtpd.Error{}) {
				err = smtpd.Error{Code: 500, Message: "Internal server error"}
			}
		} else {
			logger.Info("handled mail successfully")
		}
	}()
	env.AddReceivedLine(peer)
	peerIP := ""
	if addr, ok := peer.Addr.(*net.TCPAddr); ok {
		peerIP = addr.IP.String()
	}
	logger = logger.With(zap.String("peer", peerIP))
	logger.With(zap.String("data", string(env.Data))).Info("handling mail")

	// Abuse mails are only logged
	if strings.HasPrefix(env.Recipients[0], "abuse@") {
		w.logger.Warn("Abuse email report")
		return smtpd.Error{Code: 200, Message: "Received"}
	}

	// Sender on this server
	if peer.Username != "" {
		return w.forward(env)
	}

	// Recipients on this server
	var errs []error
	for _, rec := range env.Recipients {
		addr, err := mail.ParseAddress(rec)
		if err != nil {
			errs = append(errs, err)
			logger.Warn("failed to parse recipient", zap.String("recipient", rec))
			continue
		}
		if strings.HasSuffix(addr.Address, *domain) {
			if err := w.deliver(addr.Address, env); err != nil {
				errs = append(errs, errors.Wrap(err, "deliver failed"))
			}
		} else {
			// Error because we are not an open relay:
			// you must either be known by this server, or send to someone on this server
			errs = append(errs, smtpd.Error{Code: 451, Message: "Bad recipient address. We are no open relay."})
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// deliver handles inbox
func (w wrap) deliver(recipientEmail string, env smtpd.Envelope) error {
	be, err := FirestoreBackend(context.Background())
	if err != nil {
		return err
	}
	exists, err := be.Exists(recipientEmail)
	if err != nil {
		return err
	}
	if !exists {
		return smtpd.Error{Code: 451, Message: fmt.Sprintf("Bad recipient address %q", recipientEmail)}
	}

	w.logger.Debug("User exists", zap.String("recipient", recipientEmail))
	if err = os.MkdirAll(path.Join("mails", emailUserName(recipientEmail)), 0777); err != nil {
		return errors.Wrap(err, "failed to make inbox")
	}

	u, err := store.NewUser(path.Join("mails", emailUserName(recipientEmail)), emailUserName(recipientEmail), "")
	if err != nil {
		return err
	}

	// TODO subtract payment & forward, or place in quarantine & bounce
	isPaid := false

	if !isPaid {
		bounce := smtpd.Envelope{
			Sender:     "info@" + *domain,
			Recipients: []string{env.Sender},
			Data: []byte(fmt.Sprintf("From: %s\r\n"+
				"To: %s\r\n"+
				"Subject: E-mail requires payment\r\n"+
				"Date: %s\r\n"+
				"Message-ID: <0000000@localhost/>\r\n"+
				"Content-Type: text/plain\r\n"+
				"\r\n"+
				"You need to pay first", "info@"+*domain, env.Sender, time.Now().Format(time.RFC1123Z)))}
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
		defer cancel()
		if err = w.emit(ctx, bounce); err != nil {
			w.logger.Error("Failed to bounce for payment", zap.String("source", env.Sender), zap.Error(err))
		}
	}

	var mb backend.Mailbox
	if isPaid {
		mb, err = ensureMailbox(u, "INBOX", w.logger)
	} else {
		mb, err = ensureMailbox(u, "UNPAID", w.logger)
	}
	if err != nil {
		return err
	}
	return mb.CreateMessage(nil, time.Now(), envelopeLiteral{bytes.NewReader(env.Data), len(env.Data)})
}

func ensureMailbox(u backend.User, box string, logger *zap.Logger) (mb backend.Mailbox, err error) {
	if box == "" {
		return nil, os.MkdirAll(path.Join("mails", u.Username()), 0777)
	}
	err = os.MkdirAll(path.Join("mails", u.Username(), box), 0777)
	if err == nil {
		err = u.CreateMailbox(box)
		if err.Error() == "Mailbox already exists" {
			err = nil
		}
	}
	if err == nil {
		mb, err = u.GetMailbox(box)
	}
	if err != nil {
		logger.Warn("Failed to create mailbox", zap.String("source", u.Username()), zap.Error(err), zap.String("mailbox", box))
	}
	return
}

// forward handles outbox
func (w wrap) forward(env smtpd.Envelope) error {
	err := w.dkim(&env)
	if err != nil {
		return errors.Wrap(err, "failed to generated DKIM signer")
	}

	// Deliver to external
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
	defer cancel()
	return w.emit(ctx, env)
}

// DKIM
func (w wrap) dkim(env *smtpd.Envelope) error {
	signer, err := w.getSigner()
	if err != nil {
		return err
	}
	_, err = signer.Write(env.Data)
	if err != nil {
		return err
	}
	err = signer.Close()
	if err != nil {
		return err
	}
	PrefixLine(env, []byte(signer.Signature()))
	return nil
}

func generateUUID() string {
	uniqueID := uuid.NewRandom()
	return uniqueID.String()
}

type envelopeLiteral struct {
	io.Reader
	len int
}

func (e envelopeLiteral) Len() int {
	return e.len
}

func (e envelopeLiteral) Read(b []byte) (n int, err error) {
	return e.Reader.Read(b)
}
