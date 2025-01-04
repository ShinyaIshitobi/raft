package raft

import (
	"context"
	"log"
	"time"

	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (n *Node) sendAppendEntries(peer Peer) (int32, bool) {
	// establish connection to peer
	conn, err := grpc.NewClient(
		peer.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithIdleTimeout(5*time.Second),
	)
	if err != nil {
		log.Printf("Node %d: failed to establish connection to peer %d: %v\n", n.id, peer.id, err)
		return 0, false
	}
	defer conn.Close()

	client := rpcv1.NewRpcServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// get log entries to send
	n.mu.Lock()
	term := n.ps.currentTerm
	nextIndex := n.vls.nextIndex[peer.id]
	prevLogIndex := nextIndex - 1
	prevLogTerm := int32(0)
	if prevLogIndex > 0 && prevLogIndex <= int32(len(n.ps.logs)) {
		prevLogTerm = n.ps.logs[prevLogIndex-1].Term
	}
	var entries []*rpcv1.LogEntry
	for i := nextIndex; i <= int32(len(n.ps.logs)); i++ {
		entries = append(entries, &n.ps.logs[i-1])
	}
	leaderCommit := n.vs.commitIndex
	n.mu.Unlock()

	req := &rpcv1.AppendEntriesRequest{
		Term:         term,
		LeaderId:     n.id,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: leaderCommit,
	}
	resp, err := client.AppendEntries(ctx, req)
	if err != nil {
		log.Printf("Node %d: failed to send AppendEntries RPC to peer %s: %v\n", n.id, peer, err)
		return 0, false
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// If peer's term is greater than current term, convert to follower
	if resp.GetTerm() > n.ps.currentTerm {
		n.ps.currentTerm = resp.GetTerm()
		n.ps.votedFor = -1
		n.state = Follower
		log.Printf("Node %d: converted to Follower in Term %d\n", n.id, n.ps.currentTerm)
		return 0, false
	}

	return resp.GetTerm(), resp.GetSuccess()
}

func (n *Node) sendRequestVote(peer Peer) (int32, bool) {
	// establish connection to peer
	conn, err := grpc.NewClient(
		peer.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithIdleTimeout(5*time.Second),
	)
	if err != nil {
		log.Printf("Node %d: failed to establish connection to peer %d: %v\n", n.id, peer.id, err)
		return 0, false
	}
	defer conn.Close()

	client := rpcv1.NewRpcServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	n.mu.Lock()
	term := n.ps.currentTerm
	lastLogIndex := int32(len(n.ps.logs))
	lastLogTerm := int32(0)
	if lastLogIndex > 0 {
		lastLogTerm = n.ps.logs[lastLogIndex-1].Term
	}
	n.mu.Unlock()

	req := &rpcv1.RequestVoteRequest{
		Term:         term,
		CandidateId:  n.id,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}
	resp, err := client.RequestVote(ctx, req)
	if err != nil {
		log.Printf("Node %d: failed to send RequestVote RPC to peer %d: %v\n", n.id, peer.id, err)
		return 0, false
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	return resp.GetTerm(), resp.GetVoteGranted()
}
