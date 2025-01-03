package raft

import (
	"context"
	"log"

	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
)

func (n *Node) AppendEntries(ctx context.Context, req *rpcv1.AppendEntriesRequest) (*rpcv1.AppendEntriesResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	log.Printf("Node %d: received AppendEntries RPC from Leader %d in Term %d\n", n.id, req.LeaderId, req.Term)

	defaultResp := &rpcv1.AppendEntriesResponse{
		Term:    n.ps.currentTerm,
		Success: false,
	}

	// 1. Reply false if term < currentTerm
	if req.GetTerm() < n.ps.currentTerm {
		return defaultResp, nil
	}

	// 2. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	// If prev_log_index is 0, it means the leader has no logs.
	if req.GetPrevLogIndex() > 0 {
		if int(req.GetPrevLogIndex()) > len(n.ps.logs) {
			return defaultResp, nil
		}

		if n.ps.logs[req.GetPrevLogIndex()-1].GetTerm() != req.GetPrevLogTerm() {
			return defaultResp, nil
		}
	}

	// 3. if an existing entry conflicts with a new one (same index but different terms),
	// delete the existing entry and all that follow it.
	for i, e := range req.GetEntries() {
		index := req.GetPrevLogIndex() + int32(i) + 1 // 1-indexed
		if index <= int32(len(n.ps.logs)) {
			if n.ps.logs[index-1].GetTerm() != e.GetTerm() {
				log.Printf("Node %d: deleted log entries from index %d\n", n.id, index)
				n.ps.logs = n.ps.logs[:index-1]
				break
			}
		}
	}

	// 4. Append any new entries not already in the log
	for i, e := range req.GetEntries() {
		index := req.GetPrevLogIndex() + int32(i) + 1 // 1-indexed
		if index <= int32(len(n.ps.logs)) {
			if n.ps.logs[index-1].GetTerm() == e.GetTerm() {
				continue
			}
			n.ps.logs[index-1].Term = e.GetTerm()
			n.ps.logs[index-1].Command = e.GetCommand()
		} else {
			n.ps.logs = append(n.ps.logs, rpcv1.LogEntry{
				Term:    e.GetTerm(),
				Command: e.GetCommand(),
			})
		}
	}

	// 5. Update commitIndex
	if req.GetLeaderCommit() > n.vs.commitIndex {
		lastNewIndex := req.GetPrevLogIndex() + int32(len(req.GetEntries()))
		if req.GetLeaderCommit() < lastNewIndex {
			n.vs.commitIndex = req.GetLeaderCommit()
		} else {
			n.vs.commitIndex = lastNewIndex
		}
		log.Printf("Node %d: updated commitIndex to %d\n", n.id, n.vs.commitIndex)
	}

	// 6. Update currentTerm if needed
	if req.GetTerm() > n.ps.currentTerm {
		log.Printf("Node %d: updated currentTerm to %d\n", n.id, req.Term)
		n.ps.currentTerm = req.GetTerm()
		n.ps.votedFor = -1
		n.state = Follower
	}

	// 7. Reset election timer
	// non-blocking send
	select {
	case n.electionCh <- struct{}{}:
	default:
	}

	log.Printf("Node %d: replied AppendEntries RPC to Leader %d in Term %d\n", n.id, req.LeaderId, req.Term)
	return &rpcv1.AppendEntriesResponse{
		Term:    n.ps.currentTerm,
		Success: true,
	}, nil
}
