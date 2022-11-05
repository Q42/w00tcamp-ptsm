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
	"text/template"
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
	fb        *firestoreBackend
	getSigner func() (*dkim.Signer, error)
}

func startSmtpServers(ctx context.Context, logger *zap.Logger, tlsConfig *tls.Config, getSigner func() (*dkim.Signer, error)) {
	var servers []*smtpd.Server

	be, err := FirestoreBackend(ctx)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("error starting firestore")
	}

	for _, listen := range []protoAddr{{"starttls", ":25"}, {"starttls", ":587"}, {"tls", ":465"}} {
		var err error
		var lsnr net.Listener

		w := wrap{logger.With(zap.String("protocol", listen.protocol)), &be, getSigner}
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
			logger.Sugar().Infof("listening on address %s", listen.address)
			lsnr, err = net.Listen("tcp", listen.address)

		case "starttls":
			server.TLSConfig = tlsConfig
			server.ForceTLS = *localForceTLS

			logger.Sugar().Infof("listening on address %s (STARTTLS)", listen.address)
			lsnr, err = net.Listen("tcp", listen.address)

		case "tls":
			server.TLSConfig = tlsConfig

			logger.Sugar().Infof("listening on address %s (TLS)", listen.address)
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
		return smtpd.Error{Code: 250, Message: "Thank you."}
	}

	// Sender on this server
	if peer.Username != "" {
		return w.forward(peer, env)
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
func (w wrap) deliver(recipientEmail string, env smtpd.Envelope) (err error) {
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

	var u backend.User
	u, err = store.NewUser(path.Join("mails", emailUserName(recipientEmail)), emailUserName(recipientEmail), "")
	u = &loggingBackendUser{u, w.logger}
	if err != nil {
		return err
	}

	// TODO subtract payment & forward, or place in quarantine & bounce
	isPaid := false

	if !isPaid {
		defer func() {
			// HACK mechanism to get the createdEmail from stupid library mailbox.CreateMessage
			var createdMail noErrMailCreated
			if errors.As(err, &createdMail) {
				err = nil // reset because it was no real error
			}

			uuid := uuid.NewRandom().String()
			err = w.fb.QuarantineEmail(recipientEmail, fmt.Sprintf("%d-%s", createdMail.Uid, uuid), env)
			if err != nil {
				w.logger.Error("Failed to quarantine email at firestore", zap.String("source", env.Sender), zap.Error(err))
				return
			}

			view := template.Must(template.ParseFS(templateResources, "resources/bounce.txt"))
			buf := bytes.NewBuffer(nil)
			err = view.ExecuteTemplate(buf, "bounce.txt", map[string]interface{}{
				"Uid":             uuid,
				"Domain":          *domain,
				"From":            "info@" + *domain,
				"To":              env.Sender,
				"ReplyTo":         recipientEmail,
				"Recipients":      recipientEmail,
				"OriginalSubject": mustGetSubject(env),
				"Date":            time.Now().Format(time.RFC1123Z),
				"MailSize":        fmt.Sprintf("%dB", createdMail.Size),
				"Price":           fmt.Sprintf("$%.02f", 0.05),
				"PaymentLink":     fmt.Sprintf("https://%s/pay/%s/%d-%s", *domain, emailUserName(recipientEmail), createdMail.Uid, uuid),
			})
			if err != nil {
				w.logger.Error("Failed to create bounce email", zap.String("source", env.Sender), zap.Error(err))
			}
			bounce := smtpd.Envelope{
				Sender:     "info@" + *domain,
				Recipients: []string{env.Sender},
				Data:       []byte(buf.Bytes())}
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
			defer cancel()
			if err = w.emit(ctx, bounce); err != nil {
				w.logger.Error("Failed to bounce for payment", zap.String("source", env.Sender), zap.Error(err))
			}
		}()
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
		if err != nil && err.Error() == "Mailbox already exists" {
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
func (w wrap) forward(peer smtpd.Peer, env smtpd.Envelope) error {
	err := w.dkim(&env)
	if err != nil {
		return errors.Wrap(err, "failed to generated DKIM signer")
	}

	// Deliver to external
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
	defer cancel()

	// Move it to sent folder
	defer func() {
		var user backend.User
		user, err := store.NewUser(path.Join("mails", emailUserName(peer.Username)), emailUserName(peer.Username), peer.Password)
		if err != nil {
			w.logger.Error(err.Error(), zap.Error(err))
			return
		}
		user = &loggingBackendUser{user, w.logger}
		mb, err := ensureMailbox(user, "Sent", w.logger)
		if err != nil {
			w.logger.Error(err.Error(), zap.Error(err))
			return
		}
		err = mb.CreateMessage(
			[]string{"\\Seen"}, time.Now(),
			envelopeLiteral{bytes.NewReader(env.Data), len(env.Data)})
		if err != nil {
			w.logger.Error(err.Error(), zap.Error(err))
			return
		}
	}()

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

func mustGetSubject(env smtpd.Envelope) string {
	header, _ := getHeader(env)
	subj, _ := header.Subject()
	return subj
}

func getHeader(env smtpd.Envelope) (header mail.Header, err error) {
	defer func() {
		if err != nil {
			zap.L().Warn("Failed to read subject from envelope", zap.Error(err))
		}
	}()
	mr, err := mail.CreateReader(bytes.NewReader(env.Data))
	if err != nil {
		return
	}

	// Read each mail's parts
	for {
		_, err = mr.NextPart()
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return
		}
	}
	header = mr.Header
	return
}
