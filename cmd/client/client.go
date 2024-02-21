package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"time"

	proto "github.com/reaovyd/orcanet-market-go/internal/gen"
	"github.com/reaovyd/orcanet-market-go/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const YOUR_FILE_HERE = "go.sum"

var addr = flag.String("addr", "localhost:6699", "the address to connect to")

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
	uploader, err := c.UploadFile(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	uploader.Send(&proto.UploadFileRequest{
		PeerId:       peer_id,
		ProducerPort: "8999",
	})
	file, err := os.Open(YOUR_FILE_HERE)
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
					log.Println(resp.Peers)
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
}
