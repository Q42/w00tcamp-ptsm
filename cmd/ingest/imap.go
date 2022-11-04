package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"unsafe"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"go.uber.org/zap"
)

const (
	ImapDebug = 0
	ImapAddr  = ":993"
)

func GetUnexportedField(field reflect.Value) interface{} {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}

func startImapServers(ctx context.Context, logger *zap.Logger, tlsConfig *tls.Config) {
	be, err := FirestoreBackend(ctx)
	if err != nil {
		zap.L().Fatal(err.Error(), zap.Error(err))
	}

	// Create a new server
	s := server.New(be)
	s.Addr = ImapAddr // 143 is the insecure port
	s.AllowInsecureAuth = true
	if ImapDebug > 0 {
		s.Debug = os.Stderr
	}

	s.EnableAuth(sasl.Plain, func(conn server.Conn) sasl.Server {
		return sasl.NewPlainServer(func(identity, username, password string) error {
			logger.Info("sasl.Plain Auth", zap.String("username", username))
			if identity != "" && identity != username {
				return errors.New("identities not supported")
			}

			user, err := be.Login(conn.Info(), username, password)
			if err != nil {
				return err
			}
			ctx := conn.Context()
			ctx.State = imap.AuthenticatedState
			ctx.User = &loggingBackendUser{user, logger}
			return nil
		})
	})

	s.EnableAuth(sasl.OAuthBearer, func(conn server.Conn) sasl.Server {
		return sasl.NewOAuthBearerServer(func(opts sasl.OAuthBearerOptions) *sasl.OAuthBearerError {
			logger.Info("sasl.OAuthBearer Auth", zap.String("username", opts.Username))
			// TODO check this token!
			_ = opts.Token
			if strings.HasSuffix(opts.Username, ".q42.nl") {
				ctx := conn.Context()
				ctx.State = imap.AuthenticatedState
				user, err := be.Login(conn.Info(), "username", "password")
				if err != nil {
					return &sasl.OAuthBearerError{Status: err.Error()}
				}
				ctx.User = user
				return nil
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
	ln, err := net.Listen("tcp", ImapAddr)
	if err != nil {
		log.Fatal(err)
	}
	tlsConfig.InsecureSkipVerify = true
	tlsListener := tls.NewListener(ln, tlsConfig)
	if err := s.Serve(debugListener{tlsListener, logger.With(zap.String("ln", "tls"))}); err != nil {
		log.Fatal(err)
	}
}

type debugListener struct {
	net.Listener
	*zap.Logger
}

var _ net.Listener = debugListener{}

// Accept waits for and returns the next connection to the listener.
func (l debugListener) Accept() (conn net.Conn, err error) {
	defer func() {
		if conn != nil {
			l.Logger.Info("connection from "+conn.RemoteAddr().String(), zap.Error(err))
			conn = debugConn{conn, l.Logger.With(zap.String("remote", conn.RemoteAddr().String()))}
		}
	}()
	return l.Listener.Accept()
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l debugListener) Close() (err error) {
	return l.Listener.Close()
}

type debugConn struct {
	net.Conn
	*zap.Logger
}

var _ net.Conn = debugConn{}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (l debugConn) Read(b []byte) (n int, err error) {
	defer func() {
		if ImapDebug > 2 {
			l.Logger.Debug("read", zap.ByteString("bytes", b[0:n]), zap.Error(err))
		}
	}()
	return l.Conn.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (l debugConn) Write(b []byte) (n int, err error) {
	defer func() {
		if ImapDebug > 2 {
			l.Logger.Info("write", zap.ByteString("bytes", b[0:n]), zap.Error(err))
		}
	}()
	return l.Conn.Write(b)
}

func (l debugConn) Close() (err error) {
	defer func() {
		l.Logger.Info("close ", zap.Error(err))
	}()
	return l.Conn.Close()
}
