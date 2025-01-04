# Raft sample implementation in Go

This is a sample implementation of the Raft consensus algorithm in Go.

It is based on the paper [In Search of an Understandable Consensus Algorithm](https://raft.github.io/raft.pdf) by Diego
Ongaro and John Ousterhout.

## Usage

To run the sample implementation, you can use the following command:

```bash
go run ./cmd/server -id=1 -addr=localhost:8001 -peers=1-localhost:8001,2-localhost:8002,3-localhost:8003
go run ./cmd/server -id=2 -addr=localhost:8002 -peers=1-localhost:8001,2-localhost:8002,3-localhost:8003
go run ./cmd/server -id=3 -addr=localhost:8003 -peers=1-localhost:8001,2-localhost:8002,3-localhost:8003
```

This will start three Raft nodes on your local machine. Each node will listen on a different port and will be able to
communicate with the other nodes.

You can then use the following command to send a message to the leader:

Then, if you stop the leader, you can see that a new leader will be elected and you can send a message to the new
leader.

## Current Implementation

The current Raft implementation encompasses the following features:

1. **Node State Transitions**:

- **Follower, Candidate, Leader States**: Nodes can transition between `Follower`, `Candidate`, and `Leader` states
  based on Raft protocol rules.
- **Election Timeout**: Implements randomized election timeouts to trigger state transitions from `Follower` to
  `Candidate`.

2. **RPC Mechanisms**:

- **RequestVote RPC**:
  - Candidates can solicit votes from peer nodes to gain leadership.
  - Handles vote granting based on term and log consistency.
- **AppendEntries RPC**:
  - Leaders send heartbeats to followers to maintain authority.
  - Handles log replication and consistency checks.

## Future work

The following enhancements and features are planned for future development to improve the Raft implementation:

1. **Log Commit and Application Implementatio**n:

- **Command Reception and Log Appending**: Enable the leader to receive commands from clients, append them to its log,
  and replicate these entries to follower nodes.
- **Commit Index Update and Log Application**: Implement the logic for updating the commit index based on replicated
  logs and apply committed log entries to the state machine to reflect changes in the system's state.

2. **Enhanced Error Handling and Retry Logic**:

- **RPC Failure Management**: Add robust error handling to manage failures in RPC calls, ensuring that transient issues
  do not compromise the system's integrity.
- **Retry Mechanism**s: Implement retry strategies for failed RPC calls and network interruptions to enhance the
  resilience and reliability of inter-node communication.

3. **Client Interface Implementation**:

- **External Client Request Handling**: Develop a comprehensive client interface that can accept requests from external
  clients and appropriately redirect them to the current leader.
- **Graceful Server Shutdown**: Incorporate mechanisms for gracefully shutting down the server, ensuring that ongoing
  processes are completed, resources are released properly, and no data loss occurs during termination.

4. **Persistence**:

- **State Persistence**: Implement disk-based storage for Raft's persistent state, including the current term, voted-for
  candidate, and log entries.
- **State Restoration**: Ensure that nodes can restore their persistent state from disk upon restart, maintaining
  continuity and consistency across the cluster.

5. **Joint Consensus Implementation**:

- **Cluster Configuration Changes**: Implement joint consensus to safely add or remove nodes from the cluster without
  disrupting availability or consistency.
- **Transition Phases**: Manage the transition between old and new cluster configurations in two phases to ensure that a
  majority of nodes agree on the new configuration before finalizing changes.
