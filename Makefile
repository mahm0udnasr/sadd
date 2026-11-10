# Usage:
#   make            build ./sadd
#   make test       run tests and vet
#   sudo make install     install to /usr/local/bin and enable the boot service
#   sudo make uninstall   remove both (your hosts block is left alone)
#
# DESTDIR is for packagers; with it set, systemctl is not called.

BINARY  := sadd
PREFIX  ?= /usr/local
BINDIR  := $(PREFIX)/bin
UNITDIR ?= /etc/systemd/system
UNIT    := sadd-expire.service

# Version from git tags ("v1.0.0", "v1.0.0-3-gabc123"), or "dev" without git.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/mahm0udnasr/sadd/cmd.version=$(VERSION)

.PHONY: build test install uninstall clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go vet ./...
	go test ./...

# Install depends on the file, not on "build": under sudo, root may not have
# Go in its PATH, so build as your user first ("make"), then "sudo make install".
install: $(BINARY)
	install -Dm755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	sed 's|@BINDIR@|$(BINDIR)|g' packaging/$(UNIT) > $(UNIT).tmp
	install -Dm644 $(UNIT).tmp $(DESTDIR)$(UNITDIR)/$(UNIT)
	rm -f $(UNIT).tmp
ifeq ($(DESTDIR),)
	systemctl daemon-reload
	systemctl enable $(UNIT)
	@echo "Installed $(BINDIR)/$(BINARY). Try: sadd --help"
endif

uninstall:
ifeq ($(DESTDIR),)
	-systemctl disable $(UNIT)
endif
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY) $(DESTDIR)$(UNITDIR)/$(UNIT)
ifeq ($(DESTDIR),)
	systemctl daemon-reload
	@echo "Removed. Blocked sites are still in /etc/hosts; see README to clean up."
endif

$(BINARY):
	@echo "Build first as your normal user: make" && exit 1

clean:
	rm -f $(BINARY) $(UNIT).tmp
