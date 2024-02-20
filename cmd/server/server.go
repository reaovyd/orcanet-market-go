package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	util "github.com/reaovyd/orcanet-market-go/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/emptypb"
)

var port = flag.Int("port", 6699, "The server port")

type server struct {
	proto.UnimplementedMarketServer
}

func (s *server) RegisterPeerNode(ctx context.Context, in *emptypb.Empty) (*proto.RegisterPeerReply, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Println("Got message from ip: ", p.Addr.String())
	}
	peer_id, err := util.GeneratePeerNodeID(p.Addr.String())
	if err != nil {
		return nil, err
	}
	return &proto.RegisterPeerReply{
		PeerId: peer_id,
	}, nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	proto.RegisterMarketServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
