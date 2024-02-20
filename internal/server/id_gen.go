package server

import (
	"crypto/sha256"
	"fmt"

	"github.com/google/uuid"
)

func GeneratePeerNodeID(ip_addr string) (string, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	peer_id := uuid.String() + ip_addr
	hash := sha256.New()
	hash.Write([]byte(peer_id))
	bs := hash.Sum(nil)

	return fmt.Sprintf("%x", bs), nil
}
