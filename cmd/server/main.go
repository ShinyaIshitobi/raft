package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/ShinyaIshitobi/raft/internal/raft"
	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
	"google.golang.org/grpc"
)

func main() {
	id := flag.Int("id", 1, "ID of the node")
	addr := flag.String("addr", "localhost:8001", "Address of the node (e.g., localhost:8001)")
	peersStr := flag.String("peers", "", "Comma-separated list of peer nodes in format id:addr (e.g., 2:localhost:8002,3:localhost:8003)")
	flag.Parse()

	if *peersStr == "" {
		log.Fatalf("Peers must be specified using -peers flag")
	}

	peers := parsePeers(*peersStr)

	filteredPeers := filterSelf(*id, peers)

	node := raft.NewNode(int32(*id), *addr, filteredPeers)

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *addr, err)
	}

	grpcServer := grpc.NewServer()
	rpcv1.RegisterRpcServiceServer(grpcServer, node)

	node.Start()

	log.Printf("Node %d started at %s\n", *id, *addr)
	log.Printf("Peers: %v\n", filteredPeers)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

func parsePeers(peersStr string) []raft.Peer {
	peers := make([]raft.Peer, 0)
	peerEntries := strings.Split(peersStr, ",")
	for _, entry := range peerEntries {
		parts := strings.Split(entry, ":")
		if len(parts) != 2 {
			log.Fatalf("Invalid peer format: %s. Expected format id:addr", entry)
		}
		var id int
		_, _ = fmt.Sscanf(parts[0], "%d", &id)
		peer := raft.NewPeer(int32(id), parts[1])
		peers = append(peers, peer)
	}
	return peers
}

func filterSelf(selfID int, peers []raft.Peer) []raft.Peer {
	filtered := make([]raft.Peer, 0)
	for _, peer := range peers {
		if peer.ID() != int32(selfID) {
			filtered = append(filtered, peer)
		}
	}
	return filtered
}
