package main

import "github.com/chrj/smtpd"

func PrefixLine(env *smtpd.Envelope, line []byte) {
	env.Data = append(env.Data, line...)
	copy(env.Data[len(line):], env.Data[0:len(env.Data)-len(line)])
	copy(env.Data, line)
}
