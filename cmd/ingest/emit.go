package main

import (
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/chrj/smtpd"
)

var (
	ports = []int{25, 2525, 587}
)

// copyright: https://github.com/nilslice/email/blob/master/email.go
func (w wrap) emit(env smtpd.Envelope) error {
	for _, r := range env.Recipients {
		addr, err := mail.ParseAddress(r)
		if err != nil {
			continue
		}

		host := strings.Split(addr.Address, "@")[1]
		addrs, err := net.LookupMX(host)
		if err != nil {
			return err
		}

		c, err := newClient(addrs, ports)
		if err != nil {
			return err
		}

		err = send(env, c, r)
		if err != nil {
			return err
		}

	}

	return nil
}

func newClient(mx []*net.MX, ports []int) (*smtp.Client, error) {
	for i := range mx {
		for j := range ports {
			server := strings.TrimSuffix(mx[i].Host, ".")
			hostPort := fmt.Sprintf("%s:%d", server, ports[j])
			client, err := smtp.Dial(hostPort)
			if err != nil {
				if j == len(ports)-1 {
					return nil, err
				}

				continue
			}

			return client, nil
		}
	}

	return nil, fmt.Errorf("Couldn't connect to servers %v on any common port.", mx)
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
