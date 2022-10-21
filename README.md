# w00tcamp-PTSM
Not SMTP, but PTSM (Pay To Spam Me).

Stop spam &amp; get your deserved financial reward for reading mails. Let senders pay for emailing you.

![architecture](./docs/assets/arch.png)

## References / credits to
1. https://github.com/decke/smtprelay
1. https://cloud.google.com/compute/docs/containers/deploying-containers

## Local running

```bash
export SERVER=www.mydom.com
export CORP=mycorp
export COUNTRY=US
openssl req -x509 -sha256 -nodes -days 3650 -newkey rsa:2048 -keyout private.pem -out certificate.crt -subj "/CN=$SERVER/O=$CORP/C=$COUNTRY"

make build
./bin/ingest-darwin-arm64 -local_cert certificate.crt -local_key private.pem -hostname mail.mydom.com
```