package raft

import (
	"context"
	"log"
	"time"

	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (n *Node) sendAppendEntries(peer Peer) {
	// establish connection to peer
	conn, err := grpc.NewClient(
		peer.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithIdleTimeout(5*time.Second),
	)
	if err != nil {
		log.Printf("Node %d: failed to establish connection to peer %d: %v\n", n.id, peer.id, err)
		n.appendEntriesResultCh <- AppendEntriesResult{
			peer:    peer,
			term:    0,
			success: false,
			err:     err,
		}
		return
	}
	defer conn.Close()

	client := rpcv1.NewRpcServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// get log entries to send
	n.mu.Lock()
	term := n.ps.currentTerm
	nextIndex, ok := n.vls.nextIndex[peer.id]
	if !ok {
		nextIndex = int32(len(n.ps.logs) + 1)
		n.vls.nextIndex[peer.id] = nextIndex
	}
	prevLogIndex := nextIndex - 1
	prevLogTerm := int32(0)
	if prevLogIndex > 0 && prevLogIndex <= int32(len(n.ps.logs)) {
		prevLogTerm = n.ps.logs[prevLogIndex-1].GetTerm()
	}
	entries := make([]*rpcv1.LogEntry, 0)
	if int(prevLogIndex) < len(n.ps.logs) {
		for i := prevLogIndex; i < int32(len(n.ps.logs)); i++ {
			entry := &rpcv1.LogEntry{
				Term:    n.ps.logs[i].GetTerm(),
				Command: n.ps.logs[i].GetCommand(),
			}
			entries = append(entries, entry)
		}
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
		n.appendEntriesResultCh <- AppendEntriesResult{
			peer:    peer,
			term:    0,
			success: false,
			err:     err,
		}
		return
	}

	n.appendEntriesResultCh <- AppendEntriesResult{
		peer:    peer,
		term:    resp.GetTerm(),
		success: resp.GetSuccess(),
		err:     nil,
	}
}

func (n *Node) sendRequestVote(peer Peer) {
	// establish connection to peer
	conn, err := grpc.NewClient(
		peer.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithIdleTimeout(5*time.Second),
	)
	if err != nil {
		log.Printf("Node %d: failed to establish connection to peer %d: %v\n", n.id, peer.id, err)
		n.requestVoteResultCh <- RequestVoteResult{
			peer:        peer,
			term:        0,
			voteGranted: false,
			err:         err,
		}
		return
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
		lastLogTerm = n.ps.logs[lastLogIndex-1].GetTerm()
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
		n.requestVoteResultCh <- RequestVoteResult{
			peer:        peer,
			term:        0,
			voteGranted: false,
			err:         err,
		}
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.requestVoteResultCh <- RequestVoteResult{
		peer:        peer,
		term:        resp.GetTerm(),
		voteGranted: resp.GetVoteGranted(),
		err:         nil,
	}
}
