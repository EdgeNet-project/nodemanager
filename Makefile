BINARY=edgenet-agent
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=build

.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/agent

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
	install -m 644 systemd/edgenet-agent.service /etc/systemd/system/edgenet-agent.service
	systemctl daemon-reload

.PHONY: packages
packages: build
	@echo "Creating rpm and deb packages..."
	# Placeholder for nfpm or similar tool
	# nfpm pkg --target .

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: sync
sync:
	rsync --update --delete --exclude=.git -rv . 10.0.10.139:node-agent/