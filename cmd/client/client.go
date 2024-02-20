package main

import (
	"context"
	"flag"
	"log"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

var addr = flag.String("addr", "localhost:6699", "the address to connect to")

func main() {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := proto.NewMarketClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.RegisterPeerNode(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatalf("could not make a request to register self: %v", err)
	}
	log.Println("My registered ID is: ", r.PeerId)
}
