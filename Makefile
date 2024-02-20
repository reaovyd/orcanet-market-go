GO := go
BIN := bin
BIN_SRC_DIR := cmd

.PHONY: all clean proto clean_proto
all: proto $(BIN)/server $(BIN)/client

$(BIN)/server: $(BIN_SRC_DIR)/server/server.go
	mkdir -p $(BIN)
	$(GO) build -o $@ $<

$(BIN)/client: $(BIN_SRC_DIR)/client/client.go
	mkdir -p $(BIN)
	$(GO) build -o $@ $<

proto:
	./scripts/build.sh

clean_proto:
	./scripts/clean_proto.sh

clean:
	rm -r $(BIN)
