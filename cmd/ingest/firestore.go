package main

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"github.com/bcampbell/tameimap/store"
	"github.com/chrj/smtpd"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/backend/memory"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func FirestoreAuthenticator(ctx context.Context, logger *zap.Logger, peer smtpd.Peer, username, password string) error {
	db, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		return err
	}
	it := db.Collection("mailboxes").Doc(username).Collection("appkeys").Documents(ctx)
	for {
		doc, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		data := doc.Data()
		if data["version"] == "v1" && data["key"] == password {
			logger.Info("Logged in", zap.String("username", username))
			return nil
		}
	}
	logger.Warn("Login not found", zap.String("username", username))
	return fmt.Errorf("not found")
}

func FirestoreBackend(ctx context.Context) (firestoreBackend, error) {
	db, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		return firestoreBackend{}, err
	}
	return firestoreBackend{db, ctx}, nil
}

func (b firestoreBackend) FindUser(webLogin string) (mail string, err error) {
	doc, err := b.db.Collection("mailboxes").Query.Where("user", "==", webLogin).Documents(b.ctx).Next()
	if err != nil {
		return "", err
	}
	zap.L().Info("Found user", zap.String("mail", doc.Ref.ID))
	return doc.Ref.ID, nil
}

func (b firestoreBackend) AddAppKey(mail string, key string) (err error) {
	_, _, err = b.db.Collection("mailboxes").Doc(mail).Collection("appkeys").Add(b.ctx, map[string]interface{}{
		"key":     key,
		"date":    time.Now(),
		"version": "v1",
	})
	return err
}

func (b firestoreBackend) Exists(mail string) (exists bool, err error) {
	_, err = b.db.Collection("mailboxes").Doc(mail).Get(b.ctx)
	if err == nil {
		return true, nil
	}
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	return false, err
}

type firestoreBackend struct {
	db  *firestore.Client
	ctx context.Context
}

// Login implements backend.Backend
func (b firestoreBackend) Login(connInfo *imap.ConnInfo, username string, password string) (backend.User, error) {
	docs := b.db.Collection("mailboxes").Doc(username).Collection("appkeys").Documents(b.ctx)
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			break
		}
		data := doc.Data()
		if data["version"] == "v1" && data["key"] == password {
			var u backend.User
			u, err := store.NewUser(path.Join("mails", emailUserName(username)), emailUserName(username), password)
			u = &loggingBackendUser{u, zap.L()}
			if err != nil {
				return nil, err
			}
			return u, nil
		}
	}
	return nil, backend.ErrInvalidCredentials
}

func (b firestoreBackend) QuarantineEmail(receipientEmail string, id string, env smtpd.Envelope) (err error) {
	data := map[string]interface{}{"sender": env.Sender, "date": time.Now(), "subject": mustGetSubject(env)}
	if utf8.ValidString(string(env.Data)) {
		data["data"] = string(env.Data)
	}
	_, err = b.db.Collection("mailboxes").Doc(receipientEmail).Collection("emails").Doc(id).Create(context.Background(), data)
	return err
}

var _ backend.Backend = firestoreBackend{}

type firestoreUserBackend struct {
	db        *firestore.Client
	ctx       context.Context
	email     string
	mailboxes map[string][]*memory.Message
}

// CreateMailbox implements backend.User
func (u firestoreUserBackend) CreateMailbox(name string) error {
	return fmt.Errorf("not implemented")
}

// DeleteMailbox implements backend.User
func (u firestoreUserBackend) DeleteMailbox(name string) error {
	return fmt.Errorf("not implemented")
}

// GetMailbox implements backend.User
func (u firestoreUserBackend) GetMailbox(name string) (backend.Mailbox, error) {
	_, hasMailbox := u.mailboxes[name]
	if !hasMailbox {
		return nil, fmt.Errorf("not found")
	}
	// return m, nil
	return nil, nil
}

// ListMailboxes implements backend.User
func (u firestoreUserBackend) ListMailboxes(subscribed bool) (out []backend.Mailbox, err error) {
	for _, _ = range u.mailboxes {
		// out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return strings.Compare(out[i].Name(), out[j].Name()) >= 0 })
	return out, nil
}

// Logout implements backend.User
func (u firestoreUserBackend) Logout() error {
	return nil
}

// RenameMailbox implements backend.User
func (u firestoreUserBackend) RenameMailbox(existingName string, newName string) error {
	return fmt.Errorf("not implemented")
}

// Username implements backend.User
func (u firestoreUserBackend) Username() string {
	return u.email
}

var _ backend.User = firestoreUserBackend{}

type firebaseUserMailbox []*memory.Message

// Check implements backend.Mailbox
func (*firebaseUserMailbox) Check() error {
	panic("unimplemented")
}

// CopyMessages implements backend.Mailbox
func (*firebaseUserMailbox) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {
	panic("unimplemented")
}

// CreateMessage implements backend.Mailbox
func (*firebaseUserMailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	panic("unimplemented")
}

// Expunge implements backend.Mailbox
func (*firebaseUserMailbox) Expunge() error {
	panic("unimplemented")
}

// Info implements backend.Mailbox
func (*firebaseUserMailbox) Info() (*imap.MailboxInfo, error) {
	panic("unimplemented")
}

// ListMessages implements backend.Mailbox
func (*firebaseUserMailbox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	panic("unimplemented")
}

// Name implements backend.Mailbox
func (*firebaseUserMailbox) Name() string {
	panic("unimplemented")
}

// SearchMessages implements backend.Mailbox
func (*firebaseUserMailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	panic("unimplemented")
}

// SetSubscribed implements backend.Mailbox
func (*firebaseUserMailbox) SetSubscribed(subscribed bool) error {
	panic("unimplemented")
}

// Status implements backend.Mailbox
func (*firebaseUserMailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	panic("unimplemented")
}

// UpdateMessagesFlags implements backend.Mailbox
func (*firebaseUserMailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, operation imap.FlagsOp, flags []string) error {
	panic("unimplemented")
}

var _ backend.Mailbox = &firebaseUserMailbox{}

var body = "From: contact@example.org\r\n" +
	"To: contact@example.org\r\n" +
	"Subject: A little message, just for you\r\n" +
	"Date: Wed, 11 May 2016 14:31:59 +0000\r\n" +
	"Message-ID: <0000000@localhost/>\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\n" +
	"Hi there :)"

func unused(b *firestoreBackend) {
	var username string
	// TODO implement full spec
	_ = firestoreUserBackend{b.db, b.ctx, username, map[string][]*memory.Message{
		"INBOX": {
			{
				Uid:   6,
				Date:  time.Now(),
				Flags: []string{"\\Seen"},
				Size:  uint32(len(body)),
				Body:  []byte(body),
			},
		},
	}}

	_, _ = memory.New().Login(nil, "username", "password")
}
