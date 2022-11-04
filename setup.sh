gcloud services enable compute.googleapis.com --project=$GCLOUD_PROJECT
gcloud services enable cloudbuild.googleapis.com --project=$GCLOUD_PROJECT

gcloud compute instances create dummy --project=$GCLOUD_PROJECT \
    --zone=europe-west1-b --machine-type=e2-small --tags=smtp \
    --serviceAccount=ptsm-vm@$GCLOUD_PROJECT.iam.gserviceaccount.com \
    --create-disk=auto-delete=yes,boot=yes,device-name=dummy,image=projects/debian-cloud/global/images/debian-11-bullseye-v20220920,mode=rw,size=10,type=projects/$GCLOUD_PROJECT/zones/us-central1-a/diskTypes/pd-balanced \
    --no-shielded-secure-boot --shielded-vtpm --shielded-integrity-monitoring --reservation-affinity=any

gcloud compute --project=$GCLOUD_PROJECT firewall-rules create mail --direction=INGRESS --priority=1000 --network=default --action=ALLOW --rules=tcp:25,tcp:143,tcp:456,tcp:587,tcp:993 --source-ranges=0.0.0.0/0 --target-tags=smtp
gcloud compute --project=$GCLOUD_PROJECT firewall-rules create mailv6 --direction=INGRESS --priority=1000 --network=default --action=ALLOW --rules=tcp:25,tcp:143,tcp:456,tcp:587,tcp:993 --source-ranges=0::0/0 --target-tags=smtp
gcloud projects add-iam-policy-binding $GCLOUD_PROJECT --project=$GCLOUD_PROJECT --member serviceAccount:ptsm-vm@$GCLOUD_PROJECT.iam.gserviceaccount.com --role=roles/datastore.user

# make clean build scp
# setup
sudo apt-get install nginx certbot python3-certbot-nginx

# start
sudo ./ingest-linux-amd64 -local_cert /etc/letsencrypt/live/mail.ptsm.q42.com/fullchain.pem --local_key /etc/letsencrypt/live/mail.ptsm.q42.com/privkey.pem -hostname mail.ptsm.q42.com -domain ptsm.q42.com -local_forcetls
curl https://mail.ptsm.q42.com # requests cert
sudo ./ingest-linux-amd64 -local_cert /etc/letsencrypt/live/mail.pay2mail.me/fullchain.pem --local_key /etc/letsencrypt/live/mail.pay2mail.me/privkey.pem -hostname mail.pay2mail.me -domain ptsm.q42.com -local_forcetls
curl https://mail.pay2mail.me # requests cert

openssl s_client -showcerts -connect mail.ptsm.q42.com:993 -servername mail.ptsm.q42.com
openssl s_client -starttls smtp -showcerts -connect mail.ptsm.q42.com:25 -servername mail.ptsm.q42.com
# DANE (https://blog.zimbra.com/2022/04/zimbra-skillz-enable-dane-verification-for-incoming-email-in-zimbra/)
printf '_25._tcp.%s. IN TLSA 3 1 1 %s\n' \
        mail.ptsm.q42.com \
        $(openssl x509 -in /etc/autocert/live/mail.ptsm.q42.com -noout -pubkey |
            openssl pkey -pubin -outform DER |
            openssl dgst -sha256 -binary |
            hexdump -ve '/1 "%02x"')

# TODO ko
# TODO https://cloud.google.com/compute/docs/containers/deploying-containers
