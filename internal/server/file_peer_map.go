package server

import "sync"

type hashset map[string]bool

// remove peer on each user request on the file?
// 1. if peerid in list exists in peer_ip_map, keep. otherwise, delete
type filePeerMap struct {
	fpeer_map map[string]hashset
	lock      *sync.Mutex
}

type filePeerMapError string

func (err filePeerMapError) Error() string {
	return string(err)
}

func newFilePeerMap() filePeerMap {
	return filePeerMap{
		fpeer_map: map[string]hashset{},
		lock:      &sync.Mutex{},
	}
}

func (fpm *filePeerMap) getPeersByHash(filehash string) ([]string, error) {
	if fpm == nil {
		return nil, filePeerMapError("Object found to be null")
	}
	fpm.lock.Lock()
	set, ok := fpm.fpeer_map[filehash]
	fpm.lock.Unlock()
	if !ok {
		return nil, filePeerMapError("Could not get peers by the provided filehash")
	}
	peers := make([]string, len(set))
	i := 0
	for peer := range set {
		peers[i] = peer
	}
	return peers, nil
}

func (fpm *filePeerMap) addFileHash(filehash string, peer string) error {
	if fpm == nil {
		return filePeerMapError("Object found to be null")
	}
	fpm.lock.Lock()
	_, ok := fpm.fpeer_map[filehash]
	if !ok {
		fpm.fpeer_map[filehash] = make(hashset)
	}
	fpm.fpeer_map[filehash][peer] = true
	fpm.lock.Unlock()
	return nil
}

func (fpm *filePeerMap) removePeerByHash(filehash string, peer string) (string, error) {
	if fpm == nil {
		return "", filePeerMapError("Object found to be null")
	}
	fpm.lock.Lock()
	// if nil it's an no-op
	delete(fpm.fpeer_map[filehash], peer)
	// _, ok := fpm.fpeer_map[filehash]
	// if !ok {
	// 	fpm.lock.Unlock()
	// 	return "", filePeerMapError("Could not find the associated filehash")
	// }
	// delete(fpm.fpeer_map[filehash], peer)
	fpm.lock.Unlock()

	return peer, nil
}
