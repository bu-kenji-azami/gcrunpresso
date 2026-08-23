GIT_VER ?= $(shell git describe --tags | sed -e 's/-/+/')
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)

.PHONY: test binary install clean

cmd/gcrunpresso/gcrunpresso: *.go cmd/gcrunpresso/*.go go.*
	cd cmd/gcrunpresso && go build -tags "no_azurerm,no_s3" -ldflags "-s -w -X github.com/kayac/gcrunpresso/v2.Version=${GIT_VER}" -trimpath

install: cmd/gcrunpresso/gcrunpresso
	install cmd/gcrunpresso/gcrunpresso `go env GOPATH`/bin/gcrunpresso

test:
	go test -race ./...

packages:
	goreleaser build --skip-validate --clean

packages-snapshot:
	goreleaser build --skip-validate --clean --snapshot

clean:
	rm -f cmd/gcrunpresso/gcrunpresso
	rm -rf dist/*

orb/publish:
	circleci orb validate orb.yml
	circleci orb publish orb.yml $(ORB_NAMESPACE)/gcrunpresso@dev:latest

orb/promote:
	circleci orb publish promote $(ORB_NAMESPACE)/gcrunpresso@dev:latest patch

image-push: dist/
	docker buildx build --platform linux/amd64,linux/arm64 \
	-t ghcr.io/kayac/gcrunpresso:$(IMAGE_VERSION) \
	--push \
	-f Dockerfile .

image-load: dist/
	docker buildx build --platform linux/amd64 \
	-t ghcr.io/kayac/gcrunpresso:$(IMAGE_VERSION) \
	--load \
	-f Dockerfile .
