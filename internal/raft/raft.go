package raft

import (
	"log"
	"math/rand"
	"sync"
	"time"

	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
)

type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

type PersistentState struct {
	currentTerm int32
	votedFor    int32
	logs        []rpcv1.LogEntry // 1-indexed
}

type VolatileState struct {
	commitIndex int32
	lastApplied int32
}

type VolatileLeaderState struct {
	nextIndex  map[int32]int32
	matchIndex map[int32]int32
}

type Peer struct {
	id   int32
	addr string
}

func NewPeer(id int32, addr string) Peer {
	return Peer{id: id, addr: addr}
}

func (p Peer) ID() int32 {
	return p.id
}

type AppendEntriesResult struct {
	peer    Peer
	term    int32
	success bool
	err     error
}

type RequestVoteResult struct {
	peer        Peer
	term        int32
	voteGranted bool
	err         error
}

type Node struct {
	id   int32
	addr string

	// Raft states
	state NodeState
	ps    PersistentState
	vs    VolatileState
	vls   VolatileLeaderState

	peers    []Peer     // List of peers
	leaderID int32      // Leader ID
	mu       sync.Mutex // Lock for the node

	electionCh            chan struct{} // Channel to restart election
	appendEntriesResultCh chan AppendEntriesResult
	requestVoteResultCh   chan RequestVoteResult

	rpcv1.UnimplementedRpcServiceServer
	rpcv1.RpcServiceClient
}

func NewNode(id int32, addr string, peers []Peer) *Node {
	return &Node{
		id:    id,
		addr:  addr,
		state: Follower,
		ps: PersistentState{
			currentTerm: 0,
			votedFor:    -1,
			logs:        make([]rpcv1.LogEntry, 0),
		},
		vs: VolatileState{
			commitIndex: 0,
			lastApplied: 0,
		},
		vls: VolatileLeaderState{
			nextIndex:  make(map[int32]int32),
			matchIndex: make(map[int32]int32),
		},
		peers:                 peers,
		electionCh:            make(chan struct{}, 1), // buffered channel
		appendEntriesResultCh: make(chan AppendEntriesResult, len(peers)),
		requestVoteResultCh:   make(chan RequestVoteResult, len(peers)),
	}
}

func (n *Node) Start() {
	go n.run()
}

func (n *Node) run() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		switch n.state {
		case Follower:
			n.runFollower()
		case Candidate:
			n.runCandidate()
		case Leader:
			n.runLeader()
		}
	}
}

func (n *Node) runFollower() {
	timeout := time.Duration(rand.Intn(150)+150) * time.Millisecond
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-n.electionCh:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(timeout)
		case <-timer.C:
			n.mu.Lock()
			n.state = Candidate
			n.mu.Unlock()
			log.Printf("Node %d: converted to Candidate in Term %d\n", n.id, n.ps.currentTerm)
			return
		}
	}
}

func (n *Node) runCandidate() {
	n.mu.Lock()
	n.ps.currentTerm++
	n.ps.votedFor = n.id
	currentTerm := n.ps.currentTerm
	votesReceived := 1
	quorum := len(n.peers)/2 + 1
	n.mu.Unlock()

	log.Printf("Node %d: starting election in Term %d\n", n.id, currentTerm)

	for _, peer := range n.peers {
		go n.sendRequestVote(peer)
	}

	timeout := time.Duration(rand.Intn(150)+150) * time.Millisecond
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case res := <-n.requestVoteResultCh:
			if res.err != nil {
				log.Printf("Node %d: failed to request vote from peer %d: %v\n", n.id, res.peer.id, res.err)
				continue
			}
			if res.term > currentTerm {
				// Demote to Follower
				n.mu.Lock()
				n.ps.currentTerm = res.term
				n.ps.votedFor = -1
				n.state = Follower
				n.mu.Unlock()
				log.Printf("Node %d: demoted to Follower in Term %d\n", n.id, res.term)
				return
			}
			if res.voteGranted {
				votesReceived++
				log.Printf("Node %d: received vote from peer %d in Term %d\n", n.id, res.peer.id, currentTerm)
				if votesReceived >= quorum {
					// Promote to Leader
					n.mu.Lock()
					n.state = Leader
					// Initialize leader state
					for _, peer := range n.peers {
						n.vls.nextIndex[peer.id] = int32(len(n.ps.logs) + 1)
						n.vls.matchIndex[peer.id] = 0
					}
					n.mu.Unlock()
					log.Printf("Node %d: converted to Leader in Term %d\n", n.id, currentTerm)
					return
				}
			}
		case <-timer.C:
			n.mu.Lock()
			n.state = Candidate
			n.mu.Unlock()
			log.Printf("Node %d: starting new election in Term %d\n", n.id, currentTerm)
			return
		case <-n.electionCh:
			// Received AppendEntries RPC from Leader
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(timeout)
		}
	}
}

func (n *Node) runLeader() {
	n.mu.Lock()
	currentTerm := n.ps.currentTerm
	n.mu.Unlock()

	log.Printf("Node %d: starting to send AppendEntries RPC in Term %d\n", n.id, currentTerm)

	for _, peer := range n.peers {
		go n.sendAppendEntries(peer)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, peer := range n.peers {
				go n.sendAppendEntries(peer)
			}
		case res := <-n.appendEntriesResultCh:
			if res.err != nil {
				log.Printf("Node %d: failed to send AppendEntries RPC to peer %d: %v\n", n.id, res.peer.id, res.err)
				continue
			}
			if res.term > currentTerm {
				// Demote to Follower
				n.mu.Lock()
				n.ps.currentTerm = res.term
				n.ps.votedFor = -1
				n.state = Follower
				n.mu.Unlock()
				log.Printf("Node %d: demoted to Follower in Term %d\n", n.id, res.term)
				return
			}
			if res.success {
				n.mu.Lock()
				n.vls.matchIndex[res.peer.id] = n.vls.nextIndex[res.peer.id] - 1
				n.vls.nextIndex[res.peer.id] = n.vls.matchIndex[res.peer.id] + 1
				n.mu.Unlock()
				log.Printf("Node %d: received successful AppendEntries RPC from peer %d in Term %d\n", n.id, res.peer.id, currentTerm)
				// TODO: Update commitIndex
			} else {
				n.mu.Lock()
				if n.vls.nextIndex[res.peer.id] > 1 {
					n.vls.nextIndex[res.peer.id]--
					log.Printf("Node %d: received failed AppendEntries RPC from peer %d in Term %d\n", n.id, res.peer.id, currentTerm)
				}
				n.mu.Unlock()
			}
		case <-n.electionCh:
			// Received RequestVote RPC from Candidate
			log.Printf("Node %d: received RequestVote RPC from Candidate in Term %d\n", n.id, currentTerm)
		}
	}
}

func (n *Node) getLeaderAddress() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.state == Leader {
		return n.addr
	}
	if n.leaderID != -1 {
		for _, peer := range n.peers {
			if peer.id == n.leaderID {
				return peer.addr
			}
		}
	}
	return ""
}
