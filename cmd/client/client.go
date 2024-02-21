package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr = flag.String("addr", "localhost:6699", "the address to connect to")

func main() {
	flag.Parse()
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := proto.NewMarketClient(conn)

	// way to keep infinite joinnetwork stream for now
	ctx := context.Background()

	r, err := c.JoinNetwork(ctx)
	if err != nil {
		log.Fatalf("could not make a request to register self: %v", err)
	}
	go func() {
		for {
			time.Sleep(2 * time.Second)
			r.Send(&proto.KeepAliveRequest{})
		}
	}()
	for {
		msg, err := r.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		fmt.Println("My ID is ", msg)
	}
}
