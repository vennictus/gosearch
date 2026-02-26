BINARY := gosearch
VERSION ?= dev
DIST := dist
GOOS_LIST := linux darwin windows
GOARCH_LIST := amd64 arm64

.PHONY: test race build clean cross release checksums

test:
	go test ./...

race:
	go test -race ./...

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

clean:
	rm -rf $(DIST)

cross: clean
	mkdir -p $(DIST)
	for os in $(GOOS_LIST); do \
	  for arch in $(GOARCH_LIST); do \
	    ext=""; \
	    if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
	    out="$(DIST)/$(BINARY)-$$os-$$arch$$ext"; \
	    GOOS=$$os GOARCH=$$arch go build -ldflags "-X main.version=$(VERSION)" -o $$out .; \
	  done; \
	done

checksums:
	cd $(DIST) && (sha256sum * 2>/dev/null || shasum -a 256 *) > SHA256SUMS.txt

release: cross checksums
