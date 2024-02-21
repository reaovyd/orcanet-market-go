package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"github.com/reaovyd/orcanet-market-go/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NOTE: This isn't a correct client at all. It is just used as a test on the gRPC methods for the server.
// The client code is supposed to essentially be the peer node anyways, but only using this to test out
// the market server.

const YOUR_FILE_HERE = "go.sum"

var (
	id_hash       = flag.String("id_hash", "<NIL>", "the id hash of the peer")
	addr          = flag.String("addr", "localhost:6699", "the address to connect to")
	producer_port = flag.String("producer_port", "8899", "the producer port to listen on")
	in_file       = flag.String("in_file", "go.sum", "the file to upload")
)

func main() {
	flag.Parse()
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := proto.NewMarketClient(conn)

	peer_id_chan := make(chan string, 1)

	go func() {
		// way to keep infinite joinnetwork stream for now
		// context.WithTimeout sets deadline timeout
		ctx := context.Background()

		r, err := c.JoinNetwork(ctx)
		if err != nil {
			log.Fatalf("could not make a request to register self: %v", err)
		}
		if *id_hash != "<NIL>" {
			err = r.Send(&proto.KeepAliveRequest{
				PeerId:          *id_hash,
				ProducerPort:    *producer_port,
				JoinRequestType: proto.JoinRequestType_REJOINER,
			})
		} else {
			err = r.Send(&proto.KeepAliveRequest{
				JoinRequestType: proto.JoinRequestType_JOINER,
			})
		}
		if err != nil {
			log.Fatalf("could not make a request to register self: %v", err)
		}
		found := false
		peer_id := ""
		for {
			msg, err := r.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println(err)
				break
			}
			if !found {
				peer_id = msg.GetPeerId()
				peer_id_chan <- peer_id
				found = true
			}
		}
	}()
	peer_id := <-peer_id_chan
	fmt.Println(peer_id)
	uploader, err := c.UploadFile(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	uploader.Send(&proto.UploadFileRequest{
		PeerId:       peer_id,
		ProducerPort: *producer_port,
	})
	file, err := os.Open(*in_file)
	if err != nil {
		log.Fatal(err)
	}
	bytes := make([]byte, server.CHUNK_SIZE)
	for {
		n, err := file.Read(bytes)
		if err == io.EOF {
			resp, err := uploader.CloseAndRecv()
			if err != nil {
				log.Fatal(err)
			} else {
				filehash := resp.GetFilehash()
				req := proto.DiscoverPeersRequest{
					Filehash: filehash,
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				resp, err := c.DiscoverPeers(ctx, &req)
				if err != nil {
					log.Fatal(err)
				} else {
					log.Println(filehash)
					log.Println(resp.GetPeerIds())
				}
			}
			break
		}
		if err != nil {
			log.Fatalf(uploader.CloseSend().Error())
		}
		err = uploader.Send(&proto.UploadFileRequest{
			Chunk: bytes[:n],
		})
		if err != nil {
			log.Fatalf(uploader.CloseSend().Error())
		}
	}
	select {}
}
