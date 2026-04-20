BIN_DIR := bin
CMDS := martmart

.PHONY: build run clean test lint setup

build:
	mkdir -p $(BIN_DIR)
	for cmd in $(CMDS); do go build -o $(BIN_DIR)/$$cmd ./cmd/$$cmd; done

run:
	go run ./cmd/martmart

clean:
	rm -rf $(BIN_DIR)

test:
	go test ./...

lint:
	golangci-lint run ./...

setup:
	git config core.hooksPath .githooks
	@echo "Git hooks configured."
	@echo "Install golangci-lint: https://golangci-lint.run/welcome/install/"
