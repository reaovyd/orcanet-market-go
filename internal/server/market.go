package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net"
	"sync"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"google.golang.org/grpc/peer"
)

const (
	CHUNK_SIZE = 4096
)

type MarketServer struct {
	proto.UnimplementedMarketServer
	peer_ip_map       *sync.Map
	file_peer_map     *filePeerMap
	hasher            hash.Hash
	keepalive_timeout time.Duration
	heartbeat         time.Duration
}

func NewMarketServer(keepalive_timeout time.Duration) MarketServer {
	fpm := newFilePeerMap()
	return MarketServer{
		peer_ip_map:       &sync.Map{},
		file_peer_map:     &fpm,
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

// Runs on different goroutines; need synchronization on map
func (s *MarketServer) JoinNetwork(stream proto.Market_JoinNetworkServer) error {
	p, ok := peer.FromContext(stream.Context())
	if ok {
		ip := p.Addr.String()
		peer_id, err := GeneratePeerNodeID(p.Addr.String())
		if err != nil {
			return MarketServerError(fmt.Sprint("Failed to join the market server: ", err))
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
	return MarketServerError("Node failed to join server! Could not retrieve context from peer")
}

func (s *MarketServer) UploadFile(stream proto.Market_UploadFileServer) error {
	ctx, ok := peer.FromContext(stream.Context())
	if ok {
		ip := ctx.Addr.String()
		// Initially retrieves a request and ignores the initial chunk
		// sent
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		peer_id := req.GetPeerId()
		expected_ip := s.GetPeer(peer_id)
		if expected_ip == nil {
			return MarketServerError(fmt.Sprintf("The peer_id %s cannot be found!", peer_id))
		}
		if ip != expected_ip {
			return MarketServerError(fmt.Sprintf("Peer provided ID %s and had IP %s, but saved and expected peer IP is %s", peer_id, ip, expected_ip))
		}
		// TODO: Problem here is that contents may not necessarily be 4096 bytes
		// if someone else uses some other client that we didn't implement
		// so prob need to change that in here
		var filehash []byte
		for {
			req, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			} else {
				hasher := sha256.New()
				hasher.Write(req.GetChunk())
				filehash = append(filehash, hasher.Sum(nil)...)
				hasher.Reset()
				hasher.Write(filehash)
				filehash = hasher.Sum(nil)
				hasher.Reset()
			}
		}
		producer_port := req.GetProducerPort()
		host, _, err := net.SplitHostPort(ip)
		if err != nil {
			return err
		}
		producer_ip := net.JoinHostPort(host, producer_port)
		filehash_out := fmt.Sprintf("%x", filehash)
		s.file_peer_map.addFileHash(filehash_out, producer_ip)
		return stream.SendAndClose(&proto.UploadFileResponse{
			Filehash: filehash_out,
		})
	}
	return MarketServerError("Could not retrieve context from peer!")
}

func (s *MarketServer) DiscoverPeers(ctx context.Context, req *proto.DiscoverPeersRequest) (*proto.DiscoverPeersReply, error) {
	filehash := req.GetFilehash()
	peers, err := s.file_peer_map.getPeersByHash(filehash)
	if err != nil {
		return nil, err
	}
	return &proto.DiscoverPeersReply{
		Peers: peers,
	}, nil
}
