package main

import (
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
	Logger *zap.Logger
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
		mbs[i] = &loggingBackendMailbox{mb, m.Logger.With(zap.String("mailbox", mb.Name()))}
	}
	return mbs, err
}

func (m *loggingBackendUser) GetMailbox(name string) (backend.Mailbox, error) {
	m.Logger.Info("GetMailbox", zap.String("mailbox", name))
	mb, err := m.User.GetMailbox(name)
	if err != nil && mb == nil {
		return nil, err
	}
	return &loggingBackendMailbox{mb, m.Logger.With(zap.String("mailbox", name))}, err
}

func (m *loggingBackendMailbox) Info() (out *imap.MailboxInfo, err error) {
	defer func() {
		m.Logger.Info("Info")
		switch m.Name() {
		case "Sent":
			out.Attributes = append(out.Attributes, imap.SentAttr)
		case "Drafts":
			out.Attributes = append(out.Attributes, imap.DraftsAttr)
		case "Junk":
			out.Attributes = append(out.Attributes, imap.JunkAttr)
		case "All":
			out.Attributes = append(out.Attributes, imap.AllAttr)
		case "Trash":
			out.Attributes = append(out.Attributes, imap.TrashAttr)
		}
		if m.Name() == "Sent" {
			out.Attributes = append(out.Attributes, imap.SentAttr)
		}
	}()
	return m.Mailbox.Info()
}
