package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"slices"
	"sync"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/emptypb"
)

// remove peer on each user request on the file?
// 1. if peerid in list exists in peer_ip_map, keep. otherwise, delete
type filePeerMap struct {
	fpeer_map map[string][]string
	lock      *sync.Mutex
}

type filePeerMapError string

func (err filePeerMapError) Error() string {
	return string(err)
}

func newFilePeerMap() filePeerMap {
	return filePeerMap{
		fpeer_map: map[string][]string{},
		lock:      &sync.Mutex{},
	}
}

func (fpm *filePeerMap) addFileHash(filehash string, peer string) {
	fpm.lock.Lock()
	_, ok := fpm.fpeer_map[filehash]
	if !ok {
		fpm.fpeer_map[filehash] = make([]string, 0)
	}
	fpm.fpeer_map[filehash] = append(fpm.fpeer_map[filehash], peer)
	fpm.lock.Unlock()
}

func (fpm *filePeerMap) removePeerByHash(filehash string, peer string) (string, error) {
	fpm.lock.Lock()
	lst, ok := fpm.fpeer_map[filehash]
	if !ok {
		fpm.lock.Unlock()
		return "", filePeerMapError("Could not find the associated filehash")
	}
	rem := -1
	for i := range lst {
		if lst[i] == peer {
			i = rem
		}
	}
	if rem != -1 {
		fpm.fpeer_map[filehash] = slices.Delete(lst, rem, rem+1)
		fpm.lock.Unlock()
		return peer, nil
	}
	fpm.lock.Unlock()

	return peer, filePeerMapError("Could not find the associated peer")
}

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
		hasher:            sha256.New(),
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
		var chunk []byte = nil
		for {
			req, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			} else {
				if chunk == nil {
					chunk = req.GetChunk()
				} else {
					new_bytes := req.GetChunk()
					s.hasher.Write(new_bytes)
					bs := s.hasher.Sum(nil)
					chunk = append(chunk, bs...)
				}
				s.hasher.Write(chunk)
				chunk = s.hasher.Sum(nil)
			}
		}
		producer_port := req.GetProducerPort()
		host, _, err := net.SplitHostPort(ip)
		if err != nil {
			return err
		}
		producer_ip := net.JoinHostPort(host, producer_port)
		filehash := fmt.Sprintf("%x", chunk)
		s.file_peer_map.addFileHash(filehash, producer_ip)
		return stream.SendAndClose(&proto.UploadFileResponse{
			Filehash: filehash,
		})
	}
	return MarketServerError("Could not retrieve context from peer!")
}

func (s *MarketServer) DiscoverPeers(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	s.peer_ip_map.Range(func(key, value any) bool {
		log.Printf("key %v: %v\n", key, value)
		return true
	})
	return &emptypb.Empty{}, nil
}
