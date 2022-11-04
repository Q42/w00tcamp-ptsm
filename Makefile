SRC:=$(find ./cmd -type f)

bin/ingest-darwin-amd64: $(SRC)
	GOARCH="amd64" GOOS="darwin" go build -trimpath -o bin/ingest-darwin-amd64 ./cmd/ingest

bin/ingest-darwin-arm64: $(SRC)
	GOARCH="arm64" GOOS="darwin" go build -trimpath -o bin/ingest-darwin-arm64 ./cmd/ingest

bin/ingest-linux-amd64: $(SRC)
	GOARCH="amd64" GOOS="linux" go build -trimpath -o bin/ingest-linux-amd64 ./cmd/ingest

build: bin/ingest-darwin-amd64 bin/ingest-darwin-arm64 bin/ingest-linux-amd64

clean:
	rm -rf bin/*

.PHONY: setup-ko-gcr
setup-ko-gcr:
	git clone git@github.com:GoogleCloudPlatform/cloud-builders-community.git --depth=1;
	gcloud builds submit --project=$$GCLOUD_PROJECT ./cloud-builders-community/ko --config=./cloud-builders-community/ko/cloudbuild.yaml
	rm -rf ./cloud-builders-community

scp:
	gcloud compute scp --zone "europe-west1-b" bin/ingest-linux-amd64 "dummy":.  --project "ptsm-2022"
	@echo "now ssh using $$ gcloud compute ssh --zone "europe-west1-b" "dummy"  --project "ptsm-2022""
