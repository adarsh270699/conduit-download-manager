APP_NAME=conduit
DIST_DIR=dist
LICENSE_FILES=LICENSE NOTICE THIRD_PARTY_NOTICES.md SOURCE_OFFER.md

# daemon/version/VERSION is the only place the version is written.
VERSION:=$(shell tr -d '\r\n' < daemon/version/VERSION)
COMMIT:=$(or $(GITHUB_SHA),$(shell git rev-parse HEAD 2>/dev/null),dev)
GO_LDFLAGS=-X conduit/daemon/version.Commit=$(COMMIT)
# nfpm reads VERSION from the environment.
export VERSION

# Conduit ships no aria2c. Integration tests run a real one: CONDUIT_ARIA2C when
# it is set, otherwise the pinned copy that scripts/download_aria2_binaries fetches.
ifeq ($(OS),Windows_NT)
ARIA2_FETCH=powershell -NoProfile -ExecutionPolicy Bypass -File scripts/download_aria2_binaries.ps1 -Platform Windows
ARIA2_TEST_BIN=$(CURDIR)/.cache/aria2/windows/aria2c.exe
else
ARIA2_FETCH=bash scripts/download_aria2_binaries.sh linux
ARIA2_TEST_BIN=$(CURDIR)/.cache/aria2/linux/aria2c
endif

.PHONY: version version-check all clean aria2-test-engine license-bundle build-linux build-windows build-daemon build-daemon-linux build-daemon-windows extension test test-all test-extension fmt vet check verify verify-all

all: clean extension build-daemon

# Print the version, or check that every version field agrees with VERSION.
version:
	@echo $(VERSION)

version-check:
	cd daemon && go run ./tools/release check

clean:
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)

# Fetch the pinned aria2c that integration tests use, into .cache/aria2.
aria2-test-engine:
	@if [ -z "$$CONDUIT_ARIA2C" ] && [ ! -f "$(ARIA2_TEST_BIN)" ]; then \
		echo "Fetching pinned aria2 test engine..."; \
		$(ARIA2_FETCH); \
	fi

# Copy the GPL notices next to the built binaries.
license-bundle:
	mkdir -p $(DIST_DIR)
	cp $(LICENSE_FILES) $(DIST_DIR)/
	rm -rf $(DIST_DIR)/licenses
	cp -r licenses $(DIST_DIR)/

# Headless daemon. CGO stays off. This is not the windowed app.
build-daemon: build-daemon-linux build-daemon-windows

build-daemon-linux: license-bundle
	mkdir -p $(DIST_DIR)
	cd daemon && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -ldflags="-s -w $(GO_LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-amd64 ./cmd/main.go

build-daemon-windows: license-bundle
	mkdir -p $(DIST_DIR)
	cd daemon && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	go build -ldflags="-s -w -H=windowsgui $(GO_LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/main.go

# Ubuntu 24.04 only. The window links WebKitGTK.
build-linux:
	@if [ "$$(uname -s)" != "Linux" ]; then \
		echo "build-linux must run on Ubuntu 24.04"; \
		exit 1; \
	fi
	mkdir -p $(DIST_DIR)
	cd desktop && wails build -skipbindings -skipembedcreate -platform linux/amd64 -tags webkit2_41 -ldflags "$(GO_LDFLAGS)"
	nfpm package -f packaging/nfpm.yaml -p deb -t $(DIST_DIR)/conduit_$(VERSION)_amd64.deb

# Windows only. Builds the app, extension zip, and NSIS installer with
# scripts/build_windows.ps1, the same script CI runs.
build-windows:
	@if [ "$$(uname -s)" != "Windows_NT" ] && [ "$$OS" != "Windows_NT" ]; then \
		case "$$(uname -s)" in \
			MINGW*|MSYS*|CYGWIN*) ;; \
			*) echo "build-windows must run on Windows"; exit 1 ;; \
		esac; \
	fi
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build_windows.ps1

# Package browser extension (Chrome & Firefox MV3)
extension:
	mkdir -p $(DIST_DIR)
	rm -f $(DIST_DIR)/conduit-extension.zip
	cd extension && zip -rD ../$(DIST_DIR)/conduit-extension.zip . -x "test/*"

# Unit tests
test:
	cd daemon && go test -v ./...
	cd desktop && go test ./...
	node extension/test/test_extension.js

# Unit and integration tests
test-all: aria2-test-engine
	cd daemon && CONDUIT_ARIA2C="$${CONDUIT_ARIA2C:-$(ARIA2_TEST_BIN)}" go test -v -tags=integration ./...
	node extension/test/test_extension.js

# Extension tests
test-extension:
	node extension/test/test_extension.js

# Format Go sources in place
fmt:
	cd daemon && gofmt -w .
	cd desktop && gofmt -w .

# Static analysis
vet:
	cd daemon && go vet ./...
	cd desktop && go vet ./...

# Fast local quality gate: formatting, vetting, unit tests, extension tests
check: vet test

# Full cross-platform verification suite (Linux only; needs bash, curl, fuser, zip)
verify: verify-all

verify-all:
	./scripts/verify_cross_platform.sh
