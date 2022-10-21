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