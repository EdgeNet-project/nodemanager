BINARY=nodemanager
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PKG_VERSION=$(shell echo $(VERSION) | sed 's/^v//' | grep -E '^[0-9]' || echo 0.0.0)
BUILD_DIR=build

.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/nodemanager

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY) ./cmd/nodemanager

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: install
install: build
	install -d /usr/local/bin
	install $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	install -d /etc/edgenet
	install -m 644 systemd/nodemanager.service /etc/systemd/system/nodemanager.service
	systemctl daemon-reload

.PHONY: packages
packages: build-linux
	@echo "Creating rpm and deb packages for version $(PKG_VERSION)..."
	VERSION=$(PKG_VERSION) nfpm pkg --packager deb --target $(BUILD_DIR)/
	VERSION=$(PKG_VERSION) nfpm pkg --packager rpm --target $(BUILD_DIR)/

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: sync
sync:
	rsync --update --delete --exclude=.git -rv . 10.0.10.139:nodemanager/