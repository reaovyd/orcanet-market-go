package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"github.com/reaovyd/orcanet-market-go/internal/server"
	"google.golang.org/grpc"
)

var port = flag.Int("port", 6699, "The server port")

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	market_server := server.NewMarketServer(time.Second * 10)
	proto.RegisterMarketServer(s, &market_server)
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
