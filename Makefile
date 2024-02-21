GO := go
BIN := bin
BIN_SRC_DIR := cmd
MAIN_SERVER_LOC := $(BIN_SRC_DIR)/server/server.go
MAIN_CLIENT_LOC := $(BIN_SRC_DIR)/client/client.go
INTERNAL := internal

.PHONY: all clean proto clean_proto server client
all: proto $(BIN)/server $(BIN)/client 

server: $(BIN)/server

client: $(BIN)/client

$(BIN):
	mkdir -p $(BIN)

$(BIN)/server: $(BIN) $(MAIN_SERVER_LOC)
	$(GO) build -o $@ $(MAIN_SERVER_LOC)

$(BIN)/client: $(BIN) $(MAIN_CLIENT_LOC)
	$(GO) build -o $@ $(MAIN_CLIENT_LOC)

proto:
	./scripts/build.sh

clean_proto:
	./scripts/clean_proto.sh

clean:
	rm -r $(BIN)
