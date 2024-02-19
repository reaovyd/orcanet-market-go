GO := go
BIN := bin
BIN_SRC_DIR := cmd

# TODO add rule for building the proto file
.PHONY: all clean
all: $(BIN)/server $(BIN)/client

$(BIN)/server: $(BIN_SRC_DIR)/server/server.go
	mkdir -p $(BIN)
	$(GO) build -o $@ $<

$(BIN)/client: $(BIN_SRC_DIR)/client/client.go
	mkdir -p $(BIN)
	$(GO) build -o $@ $<

clean:
	rm -r $(BIN)
