package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"text/template"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	cms "github.com/github/smimesign/ietf-cms"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var (
	//go:embed resources
	templateMobileConfig embed.FS
)

type provisionServer struct {
	*mux.Router
	TLSConfig *tls.Config
}

func NewProvisionServer() (*provisionServer, error) {
	s := &provisionServer{mux.NewRouter(), nil}
	s.HandleFunc("/provisiontest", func(w http.ResponseWriter, r *http.Request) {
		db, err := firestore.NewClient(r.Context(), firestore.DetectProjectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = firestoreBackend{db, r.Context()}.FindUser("herman@q42.nl")
		if err != nil {
			http.Error(w, "you dummy "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = firestoreBackend{db, r.Context()}.AddAppKey("herman@ptsm.q42.com", "foobar")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment;filename=imap.mobileconfig")
		err = writeMobileProvision(w, s.TLSConfig, "herman@ptsm.q42.com", "foobar")
		if err != nil {
			w.Header().Del("Content-Type")
			w.Header().Del("Content-Disposition")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	s.HandleFunc("/provision", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id_token := strings.TrimPrefix(strings.TrimPrefix(r.Form.Get("authorization"), "Bearer"), "bearer")

		app, token, err := verify(r.Context(), id_token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		db, err := app.Firestore(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userEmail, _ := token.Claims["email"].(string)
		if userEmail == "" {
			http.Error(w, "Missing email claim in token", http.StatusBadRequest)
			return
		}

		// Credentials
		email, err := firestoreBackend{db, r.Context()}.FindUser(userEmail)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		password := make([]byte, 32)
		_, err = rand.Read(password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Write new credential to Firestore
		err = firestoreBackend{db, r.Context()}.AddAppKey(email, hex.EncodeToString(password))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Flush provision data
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment;filename=imap.mobileconfig")
		err = writeMobileProvision(w, s.TLSConfig, email, hex.EncodeToString(password))
		if err != nil {
			w.Header().Del("Content-Type")
			w.Header().Del("Content-Disposition")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	return s, nil
}

func writeMobileProvision(w io.Writer, tlsConfig *tls.Config, email, password string) error {
	cert, err := tlsConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: *hostName})
	if err != nil {
		return err
	}
	certs := []*x509.Certificate{}
	for _, der := range cert.Certificate {
		c, err := x509.ParseCertificate(der)
		if err == nil {
			certs = append(certs, c)
		}
	}
	cmsWriter := signedWriter{bytes.NewBuffer(nil), certs, asSigner(cert.PrivateKey)}
	view := template.Must(template.ParseFS(templateMobileConfig, "resources/imap.mobileconfig.xml"))
	err = view.ExecuteTemplate(cmsWriter, "imap.mobileconfig.xml", map[string]interface{}{
		"AccountDescription": email,
		"AccountName":        email,
		"ContentUuid":        uuid.New().String(),
		"PlistUuid":          uuid.New().String(),
		"DisplayDescription": "PTSM",
		"DisplayName":        "PTSM",
		"EmailAccountName":   email,
		"EmailAddress":       email,
		"Identifier":         "com.q42.ptsm",
		"Organization":       "q42",
		"Imap": map[string]interface{}{
			"Hostname": *hostName,
			"Port":     993,
			"Secure":   true,
			"Username": email,
			"Password": password,
		},
		"Smtp": map[string]interface{}{
			"Hostname": *hostName,
			"Port":     25,
			"Secure":   true,
			"Username": email,
			"Password": password,
		},
	})
	if err != nil {
		return err
	}
	return cmsWriter.FlushTo(w)
}

func asSigner(pk crypto.PrivateKey) crypto.Signer {
	switch v := pk.(type) {
	case *rsa.PrivateKey:
		return v
	case *ecdsa.PrivateKey:
		return v
	case *ed25519.PrivateKey:
		return v
	}
	return nil
}

type signedWriter struct {
	*bytes.Buffer
	certs []*x509.Certificate
	key   crypto.Signer
}

var _ io.Writer = signedWriter{}

func (s signedWriter) Write(p []byte) (n int, err error) {
	return s.Buffer.Write(p)
}
func (s signedWriter) FlushTo(w io.Writer) error {
	out, err := cms.Sign(s.Bytes(), s.certs, s.key)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func verify(ctx context.Context, token string) (app *firebase.App, out *auth.Token, err error) {
	app, err = firebase.NewApp(ctx, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error initializing app")
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error initializing auth")
	}

	out, err = client.VerifyIDToken(ctx, token)
	return
}

func emailUserName(str string) string {
	idx := strings.Index(str, "@")
	if idx <= 0 {
		return str
	}
	return str[0:idx]
}
