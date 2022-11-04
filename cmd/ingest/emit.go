package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"sort"
	"strings"

	"github.com/chrj/smtpd"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	ports = []int{465, 587, 2525, 25}
)

// TODO use a PubSub queue for this
// copyright: https://github.com/nilslice/email/blob/master/email.go
func (w wrap) emit(ctx context.Context, env smtpd.Envelope) error {
	if true {
		for _, rec := range env.Recipients {
			host := strings.Split(rec, "@")[1]
			addrs, err := net.DefaultResolver.LookupMX(ctx, host)
			if err != nil {
				return errors.Wrap(err, "failed to lookup mx")
			}
			if len(addrs) == 0 {
				return errors.Wrap(err, "no mx servers")
			}
			err = smtp.SendMail(addrs[0].Host+":587", nil, env.Sender, env.Recipients, env.Data)
			if err != nil {
				return errors.Wrap(err, "failed to create outgoing connection")
			}
		}
		return nil
	}

	// Way to complicated
	for _, r := range env.Recipients {
		addr, err := mail.ParseAddress(r)
		if err != nil {
			return errors.Wrap(err, "failed to parse address")
		}

		host := strings.Split(addr.Address, "@")[1]
		addrs, err := net.DefaultResolver.LookupMX(ctx, host)
		if err != nil {
			return errors.Wrap(err, "failed to lookup mx")
		}

		c, err := newClient(ctx, addrs, ports)
		if err != nil {
			return errors.Wrap(err, "failed to create outgoing connection")
		}

		err = send(env, c, r)
		if err != nil {
			return errors.Wrap(err, "failed to send email")
		}
	}
	return nil
}

func newClient(ctx context.Context, mx []*net.MX, ports []int) (*smtp.Client, error) {
	sort.Slice(mx, func(i, j int) bool { return mx[i].Pref < mx[j].Pref })
	opts := 0
	for range mx {
		for range ports {
			opts++
		}
	}

	pos := 0
	for i := range mx {
		for j := range ports {
			pos++
			zap.S().Debugf("mx=%s port=%d", mx[i].Host, ports[j])
			server := strings.TrimSuffix(mx[i].Host, ".")
			hostPort := fmt.Sprintf("%s:%d", server, ports[j])
			var client *smtp.Client
			var err error
			var conn net.Conn
			if ports[j] == 465 {
				conn, err = tls.Dial("tcp", hostPort, &tls.Config{})
				if err == nil {
					client, err = smtp.NewClient(conn, server)
				}
			} else {
				client, err = smtp.Dial(hostPort)
				if ports[j] == 587 && err == nil {
					err = client.StartTLS(&tls.Config{})
				}
			}
			if err != nil {
				if j == len(ports)-1 {
					return nil, err
				}
				zap.S().With(zap.Error(err)).Warnf("failure sending, trying one of next %d options", opts-pos)
				continue
			}

			return client, nil
		}
	}

	return nil, fmt.Errorf("couldn't connect to servers %v on any common port", mx)
}

func send(env smtpd.Envelope, c *smtp.Client, r string) error {
	if err := c.Mail(env.Sender); err != nil {
		return err
	}

	if err := c.Rcpt(r); err != nil {
		return err
	}

	msg, err := c.Data()
	if err != nil {
		return err
	}

	_, err = msg.Write(env.Data)
	if err != nil {
		return err
	}
	err = msg.Close()
	if err != nil {
		return err
	}
	err = c.Quit()
	if err != nil {
		return err
	}
	return nil
}
