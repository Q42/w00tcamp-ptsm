gcloud services enable compute.googleapis.com --project=$GCLOUD_PROJECT
gcloud services enable cloudbuild.googleapis.com --project=$GCLOUD_PROJECT

gcloud compute instances create dummy --project=$GCLOUD_PROJECT \
    --zone=europe-west1-b --machine-type=e2-small --tags=smtp \
    --create-disk=auto-delete=yes,boot=yes,device-name=dummy,image=projects/debian-cloud/global/images/debian-11-bullseye-v20220920,mode=rw,size=10,type=projects/$GCLOUD_PROJECT/zones/us-central1-a/diskTypes/pd-balanced \
    --no-shielded-secure-boot --shielded-vtpm --shielded-integrity-monitoring --reservation-affinity=any

gcloud compute --project=$GCLOUD_PROJECT firewall-rules create mail --direction=INGRESS --priority=1000 --network=default --action=ALLOW --rules=tcp:25,tcp:143,tcp:456,tcp:587,tcp:993 --source-ranges=0.0.0.0/0 --target-tags=smtp
gcloud compute --project=$GCLOUD_PROJECT firewall-rules create mailv6 --direction=INGRESS --priority=1000 --network=default --action=ALLOW --rules=tcp:25,tcp:143,tcp:456,tcp:587,tcp:993 --source-ranges=0::0/0 --target-tags=smtp

# TODO ko
# TODO https://cloud.google.com/compute/docs/containers/deploying-containers
