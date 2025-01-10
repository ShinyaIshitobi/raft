package raft

import (
	"fmt"
	"log"
	"sync"

	rpcv1 "github.com/ShinyaIshitobi/raft/proto/rpc/v1"
)

type StateMachine struct {
	mu    sync.Mutex
	store map[string]string
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		store: make(map[string]string),
	}
}

func (sm *StateMachine) Apply(entry *rpcv1.LogEntry) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var cmd, key, value string
	_, err := fmt.Sscanf(entry.Command, "%s %s=%s", &cmd, &key, &value)
	if err != nil {
		log.Printf("StateMachine: failed to parse command '%s': %v", entry.Command, err)
		return
	}

	if cmd == "set" {
		sm.store[key] = value
		log.Printf("StateMachine: set %s=%s", key, value)
	}
}
