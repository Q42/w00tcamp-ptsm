# w00tcamp-PTSM
Not SMTP, but PTSM (Pay To Spam Me).

Stop spam &amp; get your deserved financial reward for reading mails. Let senders pay for emailing you.

![architecture](./docs/assets/arch.png)

## References / credits to
1. https://github.com/decke/smtprelay
1. https://cloud.google.com/compute/docs/containers/deploying-containers

## Local running

```bash
export SERVER=ptsm.hermanbanken.nl
export CORP=hermanbanken.nl
export COUNTRY=NL
openssl req -x509 -sha256 -nodes -days 3650 -newkey rsa:2048 -keyout private.pem -out certificate.crt -subj "/CN=$SERVER/O=$CORP/C=$COUNTRY"

make build
./bin/ingest-darwin-arm64 -local_cert certificate.crt -local_key private.pem -hostname ptsm.hermanbanken.nl
```

## PTSM.org
Hosted mailbox/forwarder
Add credit. Pay 0.001$ per forwarded email.
Set rules, domains, whitelists, etc.
Bounces for 404?
Careful with displaying emails. XSS, embedded url loading. Tracking pixels.

## IMAP/SMTP authentication using OAuth
[XOAUTH2](https://developers.google.com/gmail/imap/xoauth2-protocol) can be used with
IMAP and SMTP using the `AUTHENTICATE` command ([RFC 3501§6.2.2](https://www.rfc-editor.org/rfc/rfc3501#section-6.2.2)).

Ref: [IMAP from local file-system](https://github.com/bcampbell/tameimap) using emersion/go-imap.

[LearnDMARC](https://www.learndmarc.com/)
[Fast development with VS Code Remote Code Server](https://github.com/coder/code-server)
[`gcloud compute config-ssh --project ptsm-2022`](https://learn.canceridc.dev/cookbook/virtual-machines/using-vs-code-with-gcp-vms)
[Air live reloading: `curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s`](https://github.com/cosmtrek/air)
