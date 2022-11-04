package main

import (
	"flag"
	"net"
	"regexp"
	"time"
)

var (
	flagset = flag.NewFlagSet("smtprelay", flag.ContinueOnError)

	// config flags
	logFile          = flagset.String("logfile", "", "Path to logfile")
	logFormat        = flagset.String("log_format", "default", "Log output format")
	logLevel         = flagset.String("log_level", "info", "Minimum log level to output")
	hostName         = flagset.String("hostname", "localhost.localdomain", "Server hostname")
	welcomeMsg       = flagset.String("welcome_msg", "", "Welcome message for SMTP session")
	listenStr        = flagset.String("listen", "127.0.0.1:25 [::1]:25", "Address and port to listen for incoming SMTP")
	localCert        = flagset.String("local_cert", "", "SSL certificate for STARTTLS/TLS")
	localKey         = flagset.String("local_key", "", "SSL private key for STARTTLS/TLS")
	localForceTLS    = flagset.Bool("local_forcetls", false, "Force STARTTLS (needs local_cert and local_key)")
	readTimeoutStr   = flagset.String("read_timeout", "60s", "Socket timeout for read operations")
	writeTimeoutStr  = flagset.String("write_timeout", "60s", "Socket timeout for write operations")
	dataTimeoutStr   = flagset.String("data_timeout", "5m", "Socket timeout for DATA command")
	maxConnections   = flagset.Int("max_connections", 100, "Max concurrent connections, use -1 to disable")
	maxMessageSize   = flagset.Int("max_message_size", 10240000, "Max message size in bytes")
	maxRecipients    = flagset.Int("max_recipients", 100, "Max RCPT TO calls for each envelope")
	allowedNetsStr   = flagset.String("allowed_nets", "127.0.0.0/8 ::1/128", "Networks allowed to send mails")
	allowedSenderStr = flagset.String("allowed_sender", "", "Regular expression for valid FROM EMail addresses")
	allowedRecipStr  = flagset.String("allowed_recipients", "", "Regular expression for valid TO EMail addresses")
	allowedUsers     = flagset.String("allowed_users", "", "Path to file with valid users/passwords")
	command          = flagset.String("command", "", "Path to pipe command")
	remotesStr       = flagset.String("remotes", "", "Outgoing SMTP servers")

	// additional flags
	_           = flagset.String("config", "", "Path to config file (ini format)")
	versionInfo = flagset.Bool("version", false, "Show version information")

	// internal
	listenAddrs       = []protoAddr{}
	readTimeout       time.Duration
	writeTimeout      time.Duration
	dataTimeout       time.Duration
	allowedNets       = []*net.IPNet{}
	allowedSender     *regexp.Regexp
	allowedRecipients *regexp.Regexp
	// remotes           = []*Remote{}
)
