package peer

import (
	"net"
	"sync"
)

type hashset map[string]bool

type PeerNode struct {
	owned_files   hashset
	lock          *sync.RWMutex
	peer_id       string
	peer_ip       string
	consumer_port string
	producer_port string
}

func (p *PeerNode) SetConsumerPort(consumer_port string) {
	p.lock.Lock()
	p.consumer_port = consumer_port
	p.lock.Unlock()
}

func (p *PeerNode) GetConsumerPort() string {
	p.lock.RLock()
	port := p.consumer_port
	p.lock.RUnlock()
	return port
}

func (p *PeerNode) SetProducerPort(producer_port string) {
	p.lock.Lock()
	p.producer_port = producer_port
	p.lock.Unlock()
}

func (p *PeerNode) SetPeerIP(peer_ip string) {
	p.lock.Lock()
	p.peer_ip = peer_ip
	p.lock.Unlock()
}

func (p *PeerNode) GetProducerPort() string {
	p.lock.RLock()
	port := p.producer_port
	p.lock.RUnlock()
	return port
}

func (p *PeerNode) AddFile(file string) {
	p.lock.Lock()
	p.owned_files[file] = true
	p.lock.Unlock()
}

func (p *PeerNode) GetPeerID() string {
	p.lock.RLock()
	id := p.peer_id
	p.lock.RUnlock()
	return id
}

func (p *PeerNode) GetPeerIP() string {
	p.lock.RLock()
	peer_ip := p.peer_ip
	p.lock.RUnlock()
	return peer_ip
}

func (p *PeerNode) GetPeerHostConsumer() string {
	p.lock.RLock()
	peer_ip := p.peer_ip
	consumer_port := p.consumer_port
	p.lock.RUnlock()
	return net.JoinHostPort(peer_ip, consumer_port)
}

func (p *PeerNode) GetPeerHostProducer() string {
	p.lock.RLock()
	peer_ip := p.peer_ip
	producer_port := p.producer_port
	p.lock.RUnlock()
	return net.JoinHostPort(peer_ip, producer_port)
}

func (p *PeerNode) GetOwnedFiles() []string {
	p.lock.RLock()
	files := make([]string, len(p.owned_files))
	p.lock.RUnlock()
	i := 0
	for file := range p.owned_files {
		files[i] = file
		i++
	}

	return files
}

func New(peer_id, peer_ip string) PeerNode {
	return PeerNode{
		peer_id:     peer_id,
		peer_ip:     peer_ip,
		owned_files: make(hashset),
		lock:        &sync.RWMutex{},
	}
}
