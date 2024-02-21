package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	peer "github.com/reaovyd/orcanet-market-go/internal"
	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	google_peer "google.golang.org/grpc/peer"
)

const (
	CHUNK_SIZE = 4096
)

type MarketServer struct {
	proto.UnimplementedMarketServer
	peer_ip_map       *sync.Map
	file_peer_map     *filePeerMap
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

func (s *MarketServer) AddNewPeer(peer_id, ip, consumer_port string) error {
	_, ok := s.peer_ip_map.Load(peer_id)
	if ok {
		return MarketServerError("Peer already exists and is connected!")
	}
	peer := peer.New(peer_id, ip)
	peer.SetConsumerPort(consumer_port)
	s.peer_ip_map.Store(peer_id, &peer)
	return nil
}

func (s *MarketServer) getPeerNode(peer_id string) (*peer.PeerNode, error) {
	val, ok := s.peer_ip_map.Load(peer_id)
	if !ok {
		return nil, MarketServerError("Peer not found in map")
	}
	return val.(*peer.PeerNode), nil
}

func (s *MarketServer) UpdateExistingPeerIp(peer_id, ip string) error {
	val, err := s.getPeerNode(peer_id)
	if err != nil {
		return err
	}
	val.SetPeerIP(ip)
	return nil
}

func (s *MarketServer) UpdateExistingPeerProducerPort(peer_id, producer_port string) error {
	val, err := s.getPeerNode(peer_id)
	if err != nil {
		return err
	}
	val.SetProducerPort(producer_port)
	return nil
}

func (s *MarketServer) UpdateExistingPeerConsumerPort(peer_id, consumer_port string) error {
	val, err := s.getPeerNode(peer_id)
	if err != nil {
		return err
	}
	val.SetConsumerPort(consumer_port)
	return nil
}

func (s *MarketServer) DeleteExistingPeer(peer_id string) {
	// NOTE: Should we really even remove the peers from the owned files inside file_peer_map?
	// Probably not since the peer can just rejoin and not have to reupload the files it had stored.
	// The provided producer_port would just need to be valid.
	//
	// val, err := s.getPeerNode(peer_id)
	// if err != nil {
	// 	return err
	// }
	// owned_files := val.GetOwnedFiles()
	// for _, file := range owned_files {
	// 	s.file_peer_map.removePeerFromFile(file, peer_id)
	// }
	s.peer_ip_map.Delete(peer_id)
}

// Runs on different goroutines; need synchronization on map
// FIXIT: Might need something for rejoins since disconnects would immediately invalidate
// anything the peer uploaded
func (s *MarketServer) JoinNetwork(stream proto.Market_JoinNetworkServer) error {
	p, ok := google_peer.FromContext(stream.Context())
	if ok {
		var peer_id string
		ip := p.Addr.String()
		init_req, err := stream.Recv()
		if err != nil {
			return MarketServerError(fmt.Sprint("Failed to join the market server: ", err))
		}
		switch init_req.JoinRequestType {
		case proto.JoinRequestType_JOINER:
			fmt.Println("Joiner")
			peer_id, err = generatePeerNodeID(p.Addr.String())
		case proto.JoinRequestType_REJOINER:
			fmt.Println("REJoiner")
			peer_id, err = init_req.GetPeerId(), nil
		}

		if err != nil {
			return MarketServerError(fmt.Sprint("Failed to join the market server: ", err))
		}
		resp := &proto.KeepAliveResponse{
			PeerId: peer_id,
		}

		// Initial Send
		if err := stream.Send(resp); err != nil {
			return err
		}
		ip, consumer_port, err := net.SplitHostPort(ip)
		if err != nil {
			return err
		}
		if err = s.AddNewPeer(peer_id, ip, consumer_port); err != nil {
			return err
		}
		if init_req.JoinRequestType == proto.JoinRequestType_REJOINER {
			if err = s.UpdateExistingPeerProducerPort(peer_id, init_req.GetProducerPort()); err != nil {
				return err
			}
		}
		// It'll still keep sending peer_id and peer node can choose to ignore it

		ticker := time.NewTicker(s.heartbeat)
		defer ticker.Stop()
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
	ctx, ok := google_peer.FromContext(stream.Context())
	if ok {
		ip := ctx.Addr.String()
		// Initially retrieves a request and ignores the initial chunk
		// sent
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		peer_id := req.GetPeerId()
		node, err := s.getPeerNode(peer_id)
		if err != nil {
			return err
		}
		expected_ip := node.GetPeerHostConsumer()
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
		filehash_out := fmt.Sprintf("%x", filehash)
		producer_port := req.GetProducerPort()
		node.SetProducerPort(producer_port)
		s.file_peer_map.addFileHash(filehash_out, peer_id)
		node.AddFile(filehash_out)
		// producer_port := req.GetProducerPort()
		// host, _, err := net.SplitHostPort(ip)
		// if err != nil {
		// 	return err
		// }
		// producer_ip := net.JoinHostPort(host, producer_port)
		// filehash_out := fmt.Sprintf("%x", filehash)
		// s.file_peer_map.addFileHash(filehash_out, producer_ip)
		return stream.SendAndClose(&proto.UploadFileResponse{
			Filehash: filehash_out,
		})
	}
	return MarketServerError("Could not retrieve context from peer!")
}

func (s *MarketServer) DiscoverPeers(ctx context.Context, req *proto.DiscoverPeersRequest) (*proto.DiscoverPeersReply, error) {
	filehash := req.GetFilehash()
	peer_id_list, err := s.file_peer_map.getPeersByHash(filehash)
	if err != nil {
		return nil, err
	}
	peers := make([]string, 0)
	for _, peer_id := range peer_id_list {
		// NOTE: Filtering for peers that are still connected
		if val, err := s.getPeerNode(peer_id); val != nil && err == nil {
			peers = append(peers, val.GetPeerHostProducer())
		}
	}
	return &proto.DiscoverPeersReply{
		PeerIds: peers,
	}, nil
}
