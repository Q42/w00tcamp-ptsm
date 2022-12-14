package main

import (
	"time"

	"github.com/bcampbell/tameimap/store"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"go.uber.org/zap"
)

type loggingBackendUser struct {
	backend.User
	Logger *zap.Logger
}

type loggingBackendMailbox struct {
	backend.Mailbox
	Logger  *zap.Logger
	LastUid uint32
}

var _ backend.User = &loggingBackendUser{}
var _ backend.Mailbox = &loggingBackendMailbox{}

func (m *loggingBackendUser) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	m.Logger.Info("ListMailboxes")
	mbs, err := m.User.ListMailboxes(subscribed)
	if err != nil && mbs == nil {
		return nil, err
	}
	for i, mb := range mbs {
		mbs[i] = &loggingBackendMailbox{mb, m.Logger.With(zap.String("mailbox", mb.Name())), 0}
	}
	return mbs, err
}

func (m *loggingBackendUser) GetMailbox(name string) (backend.Mailbox, error) {
	m.Logger.Info("GetMailbox", zap.String("mailbox", name))
	mb, err := m.User.GetMailbox(name)
	if err != nil && mb == nil {
		return nil, err
	}
	return &loggingBackendMailbox{mb, m.Logger.With(zap.String("mailbox", name)), 0}, err
}

func (m *loggingBackendMailbox) Info() (out *imap.MailboxInfo, err error) {
	defer func() {
		switch m.Name() {
		case "Sent":
			out.Attributes = include(out.Attributes, imap.SentAttr)
		case "Drafts":
			out.Attributes = include(out.Attributes, imap.DraftsAttr)
		case "Junk":
			out.Attributes = include(out.Attributes, imap.JunkAttr)
		case "All":
			out.Attributes = include(out.Attributes, imap.AllAttr)
		case "Trash":
			out.Attributes = include(out.Attributes, imap.TrashAttr)
		}
		m.Logger.Info("Info", zap.Any("info", out))
	}()
	return m.Mailbox.Info()
}

func include(list []string, needle string) []string {
	for _, s := range list {
		if needle == s {
			return list
		}
	}
	return append(list, needle)
}

func (m *loggingBackendMailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) (err error) {
	defer func() {
		if mbox, isStore := m.Mailbox.(*store.Mailbox); err == nil && isStore {
			err = noErrMailCreated{mbox.Messages[len(mbox.Messages)-1]}
		}
	}()
	return m.Mailbox.CreateMessage(flags, date, body)
}

type noErrMailCreated struct {
	*store.Message
}

func (noErrMailCreated) Error() string {
	return "not an error: mail created"
}
