package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MarketServer struct {
	proto.UnimplementedMarketServer
	peer_ip_map       *sync.Map
	keepalive_timeout time.Duration
	heartbeat         time.Duration
}

func NewMarketServer(keepalive_timeout time.Duration) MarketServer {
	return MarketServer{
		peer_ip_map:       &sync.Map{},
		keepalive_timeout: keepalive_timeout,
		heartbeat:         keepalive_timeout / 2,
	}
}

func (s *MarketServer) AddNewPeer(peer_id string, ip string) {
	s.peer_ip_map.Store(peer_id, ip)
}

func (s *MarketServer) DeleteExistingPeer(peer_id string) {
	s.peer_ip_map.Delete(peer_id)
}

func (s *MarketServer) GetPeer(peer_id string) any {
	value, ok := s.peer_ip_map.Load(peer_id)
	if !ok {
		return nil
	}
	return value
}

func (s *MarketServer) JoinNetwork(stream proto.Market_JoinNetworkServer) error {
	p, ok := peer.FromContext(stream.Context())
	if ok {
		ip := p.Addr.String()
		peer_id, err := GeneratePeerNodeID(p.Addr.String())
		if err != nil {
			return MarketServerError(fmt.Sprintln("Failed to join the market server: ", err))
		}
		s.AddNewPeer(peer_id, ip)

		ticker := time.NewTicker(s.heartbeat)
		defer ticker.Stop()
		resp := &proto.KeepAliveResponse{
			PeerId: peer_id,
		}

		// Initial Send
		if err := stream.Send(resp); err != nil {
			s.DeleteExistingPeer(peer_id)
			return err
		}
		// It'll still keep sending peer_id and peer node can choose to ignore it
		for {
			select {
			case <-ticker.C:
				if err := stream.Send(resp); err != nil {
					s.DeleteExistingPeer(peer_id)
					return err
				}
			case <-stream.Context().Done():
				s.DeleteExistingPeer(peer_id)
				return stream.Context().Err()
			}
		}
	}
	return nil
}

func (s *MarketServer) DiscoverPeers(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	s.peer_ip_map.Range(func(key, value any) bool {
		log.Printf("key %v: %v\n", key, value)
		return true
	})
	return &emptypb.Empty{}, nil
}
