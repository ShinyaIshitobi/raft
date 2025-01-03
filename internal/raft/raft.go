package raft

import (
	"math/rand"
	"sync"
	"time"

	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
)

const (
	heartbeatBaseInterval = 50 * time.Millisecond
)

func heartbeatInterval() time.Duration {
	return heartbeatBaseInterval + time.Duration(rand.Intn(50))*time.Millisecond
}

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

type Node struct {
	id   int32
	addr string

	// Raft states
	state NodeState
	ps    PersistentState
	vs    VolatileState
	vls   VolatileLeaderState

	peers []string   // List of peer addresses
	mu    sync.Mutex // Lock for the node

	electionCh chan struct{} // Channel to restart election

	rpcv1.UnimplementedRpcServiceServer
	rpcv1.RpcServiceClient
}

func NewNode(id int32, addr string, peers []string) *Node {
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
		peers:      peers,
		electionCh: make(chan struct{}, 1), // buffered channel
	}
}
