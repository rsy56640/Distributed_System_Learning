package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"labgob"
	"labrpc"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var msgNo uint64 = 0

func getMsgNo() uint64 {
	return atomic.AddUint64(&msgNo, 1)
}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type ServerState int

const (
	Follower ServerState = iota
	Candidate
	Leader
)

type LogEntry struct {
	Term    int
	Index   int
	Command interface{}
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// persistent state on all servers.
	state       ServerState
	currentTerm int
	votedFor    int // -1 means no vote during current term
	logs        []LogEntry

	leader int // -1 means donot know who is leader in current term

	// record time duration
	electionTimer     *time.Timer   // election timer
	electionTimeout   time.Duration // 200~400ms
	heartbeatInterval time.Duration // 40ms

	// volatile state on all servers.
	commitIndex int
	lastApplied int
	commitCond  *sync.Cond

	// volatile state on leader, reset when the server becomes leader.
	nextIndex  []int
	matchIndex []int

	// sync util
	notLeaderCh chan struct{}
	shutDownCh  chan struct{}

	// used for test
	applyCh chan ApplyMsg // outgoing channel to service
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool

	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = rf.leader == rf.me
	return term, isleader
}

// caller should hold lock
func (rf *Raft) turnToFollower(term int) {
	if rf.state == Leader {
		rf.notLeaderCh <- struct{}{}
	}
	rf.state = Follower
	rf.currentTerm = term
	rf.votedFor = -1
	rf.leader = -1
	rf.resetTimer()
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:

	DDEBUG(PERSIST,
		"[peer=%d] persists\n",
		rf.me)

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)

	e.Encode(rf.state)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)

	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:

	DDEBUG(PERSIST,
		"[peer=%d] read persists\n",
		rf.me)

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)

	d.Decode(&rf.state)
	d.Decode(&rf.currentTerm)
	d.Decode(&rf.votedFor)
	d.Decode(&rf.logs)
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	CandidateTerm int
	CandidateID   int
	LastLogIndex  int
	LastLogTerm   int
	MsgNo         uint64
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	CurrentTerm int // used when not vote, return self term if candidate is out of date.
	VoteGranted bool
	MsgNo       uint64
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).

	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

	reply.MsgNo = args.MsgNo

	// update self if necessary
	// no need to reset timer unless me is leader !!!
	if rf.currentTerm < args.CandidateTerm {
		DDEBUG(REQUEST_VOTE_RPC,
			"[msgNo=%d], [candidate=%d] RequestVoteArgs [peer=%d], self is out, new [term=%d]\n",
			args.MsgNo, args.CandidateID, rf.me, args.CandidateTerm)
		if rf.state == Leader {
			rf.notLeaderCh <- struct{}{}
			rf.resetTimer()
		}
		rf.state = Follower
		rf.currentTerm = args.CandidateTerm
		rf.votedFor = -1
		rf.leader = -1
	}

	// reject if candidate is out of date
	if rf.currentTerm > args.CandidateTerm {
		DDEBUG(REQUEST_VOTE_RPC,
			"[msgNo=%d], [candidate=%d] RequestVoteArgs [peer=%d], candidate is out, new [term=%d]\n",
			args.MsgNo, args.CandidateID, rf.me, rf.currentTerm)
		reply.CurrentTerm = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// not voted yet or msg lost/retransmit
	if rf.votedFor == -1 || rf.votedFor == args.CandidateID {
		// compare the term and index
		var vote bool
		vote = (args.LastLogTerm > rf.logs[len(rf.logs)-1].Term)
		if args.LastLogTerm == rf.logs[len(rf.logs)-1].Term {
			vote = (args.LastLogIndex >= rf.logs[len(rf.logs)-1].Index)
		}
		if vote == true {
			DDEBUG(REQUEST_VOTE_RPC,
				"[msgNo=%d], [candidate=%d] RequestVoteArgs [peer=%d], get vote !!!\n",
				args.MsgNo, args.CandidateID, rf.me)
			reply.VoteGranted = true
			// update self, reset timer
			rf.state = Follower
			rf.currentTerm = args.CandidateTerm
			rf.votedFor = args.CandidateID
			rf.leader = args.CandidateID
			rf.resetTimer()
		} else {
			DDEBUG(REQUEST_VOTE_RPC,
				"[msgNo=%d], [candidate=%d] RequestVoteArgs [peer=%d], reject due to log safety !!!\n",
				args.MsgNo, args.CandidateID, rf.me)
			reply.VoteGranted = false
			reply.CurrentTerm = -1
			return
		}
	} else { // have voted for other, so reject
		DDEBUG(REQUEST_VOTE_RPC,
			"[msgNo=%d], [candidate=%d] RequestVoteArgs [peer=%d], reject since have voted for [peer=%d] !!!\n",
			args.MsgNo, args.CandidateID, rf.me, rf.votedFor)
		reply.VoteGranted = false
		return
	}

}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// no reset timer
func (rf *Raft) electionSuccessReset() {
	rf.state = Leader
	rf.leader = rf.me

	count := len(rf.peers)
	length := len(rf.logs)
	for i := 0; i < count; i++ {
		rf.nextIndex[i] = length
		rf.matchIndex[i] = 0
	}
	rf.matchIndex[rf.me] = length - 1
}

func (rf *Raft) candidateRequestVote() {

	rf.mu.Lock()
	rf.resetTimer()
	rf.state = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	peer_size := len(rf.peers)
	voteArg := RequestVoteArgs{
		CandidateTerm: rf.currentTerm,
		CandidateID:   rf.me,
		LastLogIndex:  rf.logs[len(rf.logs)-1].Index,
		LastLogTerm:   rf.logs[len(rf.logs)-1].Term,
		MsgNo:         getMsgNo(),
	}
	DDEBUG(CANVASS_VOTE,
		"[msgNp=%d], [candidate=%d] begin canvass, [term=%d]\n",
		voteArg.MsgNo, rf.me, rf.currentTerm)
	currentTerm := rf.currentTerm
	rf.mu.Unlock()

	vote := 1

	// maybe network delay when reply
	replyHandler := func(reply *RequestVoteReply, peerID, currentTerm int) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		// !!!!!!!!!!!
		// msg might be delayed.
		// !!!!!!!!!!!
		if rf.currentTerm > currentTerm {
			return
		}

		// update routine
		if reply.VoteGranted == false && reply.CurrentTerm > rf.currentTerm {
			DDEBUG(CANVASS_VOTE,
				"[msgNo=%d], [candidate=%d] turn to follower, new [term=%d]\n",
				reply.MsgNo, rf.me, reply.CurrentTerm)
			rf.turnToFollower(reply.CurrentTerm)
		}

		if rf.state == Candidate { // else discard
			if reply.VoteGranted == true { // vote success
				vote++
				DDEBUG(CANVASS_VOTE,
					"[msgNo=%d], [candidate=%d] has [vote=%d]\n",
					reply.MsgNo, rf.me, vote)
				if vote*2 > peer_size {
					// no need to reset timer when become leader
					DDEBUG(CANVASS_VOTE, "[candidate=%d] becomes leader\n", rf.me)
					rf.stopTimer()
					rf.electionSuccessReset()
					go rf.heartbeatRoutine()
				}
			}
		}
	}

	for i := 0; i < peer_size; i++ {
		if i == rf.me {
			continue
		}
		voteReply := RequestVoteReply{}
		go func(peerID int) {
			rf.sendRequestVote(peerID, &voteArg, &voteReply)
			replyHandler(&voteReply, peerID, currentTerm)
		}(i)
	}
}

func (rf *Raft) resetTimer() {
	DDEBUG(TIMER,
		"[peer=%d] reset timer\n",
		rf.me)
	rf.electionTimer.Stop()
	rf.electionTimer = time.AfterFunc(rf.electionTimeout, func() { rf.candidateRequestVote() })
}

func (rf *Raft) stopTimer() {
	DDEBUG(TIMER,
		"[peer=%d] stop timer\n",
		rf.me)
	rf.electionTimer.Stop()
}

func (rf *Raft) waitCommitUntil(cond func() bool) {
	for !cond() {
		rf.commitCond.Wait()
	}
}

func (rf *Raft) tryCommit() {
	rf.commitCond.Signal()
}

// execute all the time until shut down.
func (rf *Raft) applyEntryRoutine() {
	for {
		var logs []LogEntry

		select {
		case <-rf.shutDownCh:
			return
		default:
			rf.mu.Lock()
			rf.waitCommitUntil(func() bool { return rf.lastApplied < rf.commitIndex })
			logs = make([]LogEntry, rf.commitIndex-rf.lastApplied)
			copy(logs, rf.logs[rf.lastApplied+1:rf.commitIndex+1])
			rf.lastApplied = rf.commitIndex
			rf.mu.Unlock()
		}

		// apply the log entry outside
		for _, log := range logs {
			reply := ApplyMsg{
				CommandValid: true,
				CommandIndex: log.Index,
				Command:      log.Command,
			}
			rf.applyCh <- reply

			DDEBUG(APPLY_ENTRY_ROUTINE,
				"[peer=%d] apply %v to client\n", rf.me, reply)
		}
	}
}

type AppendEntryArg struct {
	Term              int
	Leader            int
	PrevLogIndex      int
	PrevLogTerm       int
	Entries           []LogEntry
	LeaderCommitIndex int
	MsgNo             uint64
}

type AppendEntryReply struct {
	// used when fail, return
	//		-1 				if log is dismatched, or
	//		self.term 		if the msg is delayed.
	Term int

	FirstIndex int // denotes the first index of the prevTerm in collision.

	Success bool
	MsgNo   uint64
}

func (rf *Raft) AppendEntry(args *AppendEntryArg, reply *AppendEntryReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

	reply.MsgNo = args.MsgNo

	// update self
	if args.Term > rf.currentTerm {
		DDEBUG(APPEND_ENTRY_RPC,
			"[msgNo=%d], append log RPC [leader=%d], [peer=%d], self is out, new [term=%d]\n",
			args.MsgNo, args.Leader, rf.me, args.Term)
		rf.turnToFollower(args.Term)
	}

	if args.Term < rf.currentTerm {
		// network delay
		// return failure and current term to notice the leader,
		// leader will find the term is same or less, then discard.
		DDEBUG(APPEND_ENTRY_RPC,
			"[msgNo=%d], append log RPC [leader=%d], [peer=%d], msg is delayed, current [term=%d]\n",
			args.MsgNo, args.Leader, rf.me, rf.currentTerm)
		reply.Success = false
		reply.Term = rf.currentTerm
	} else { // args.term == rf.currentTerm

		// maybe me is partitioned before, canvass lonely, and now reconnect.
		if rf.state == Candidate {
			rf.state = Follower
		}

		// the current leader's message, regardless of network delay.
		rf.resetTimer() // here might reset timer twice, since me.term is less.

		// attempt to append log entries
		// first check previous term and index
		if args.PrevLogIndex <= rf.logs[len(rf.logs)-1].Index &&
			args.PrevLogTerm == rf.logs[args.PrevLogIndex].Term {

			if len(args.Entries) > 1 {
				DDEBUG(APPEND_ENTRY_RPC,
					"[msgNo=%d], [peer=%d], [old.log=(%d, %d)], [append-log=(%d, %d)]\n",
					args.MsgNo, rf.me, rf.logs[len(rf.logs)-1].Term, rf.logs[len(rf.logs)-1].Index,
					args.Entries[len(args.Entries)-1].Term, args.Entries[len(args.Entries)-1].Index)
			}

			// NB: the leader's logs might be shorter than me (no vote for leader)
			// since me might be the previous leader, and have extra log.

			// append log
			rf.logs = rf.logs[0 : args.PrevLogIndex+1]
			append_size := len(args.Entries)
			for i := 0; i < append_size; i++ {
				rf.logs = append(rf.logs, args.Entries[i])
			}

			/*
				// update log
				update_size := Min(len(args.Entries), rf.logs[len(rf.logs)-1].Index-args.PrevLogIndex)
				for i := 0; i < update_size; i++ {
					rf.logs[i+args.PrevLogIndex+1] = args.Entries[i]
				}

				// append log
				append_size := len(args.Entries) - update_size
				for i := update_size; i < update_size+append_size; i++ {
					rf.logs = append(rf.logs, args.Entries[i])
				}
			*/

			// update commit index
			// ok to update,
			// since we have the log longer than leader.commit when sending this msg.
			if args.LeaderCommitIndex > rf.commitIndex {
				rf.commitIndex = args.LeaderCommitIndex
				rf.tryCommit()
			}

			reply.Success = true

			DDEBUG(APPEND_ENTRY_RPC,
				"[msgNo=%d], log consistency check ok, [leader=%d], [peer=%d], [term=%d], append log[%d:%d]\n",
				args.MsgNo, args.Leader, rf.me, args.Term,
				len(rf.logs)-append_size, len(rf.logs))

		} else { // check fail
			DDEBUG(APPEND_ENTRY_RPC,
				"[msgNo=%d], log consistency check failed, [leader=%d], [peer=%d], [term=%d], [prev=(%d,%d)]\n",
				args.MsgNo, args.Leader, rf.me, args.Term,
				args.PrevLogTerm, args.PrevLogIndex)
			reply.Term = -1
			reply.Success = false
			reply.FirstIndex = 1
			for i := 0; i < len(rf.logs); i++ {
				if rf.logs[i].Term == args.PrevLogTerm {
					reply.FirstIndex = rf.logs[i].Index
					break
				}
			}
		}
	}
}

func (rf *Raft) sendAppendEntry(server int, args *AppendEntryArg, reply *AppendEntryReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntry", args, reply)
	return ok
}

func (rf *Raft) AppendLogReplyHandler(peerID int, reply *AppendEntryReply, curIndex int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// update self, regardless whether self is leader.
	if reply.Success == false && reply.Term != -1 {
		if reply.Term > rf.currentTerm {
			DDEBUG(LEADER_APPEN_LOG_REPLY_HANDLER,
				"[msgNo=%d], [leader=%d] append log at [peer=%d] failed, term is out, new [term=%d]\n",
				reply.MsgNo, rf.me, peerID, reply.Term)
			rf.turnToFollower(reply.Term)
		}
	} // will return immediately

	// if self is still a leader
	if rf.state == Leader {

		// NB: msg might be delayed !!!
		if reply.Success == false && reply.Term == -1 { // log dismatched, but msg maybe delayed.
			DDEBUG(LEADER_APPEN_LOG_REPLY_HANDLER,
				"[msgNo=%d], [leader=%d] append log dismatched at [peer=%d]\n",
				reply.MsgNo, rf.me, peerID)

			rf.nextIndex[peerID] = Max(
				Min(rf.nextIndex[peerID]-1, reply.FirstIndex-1),
				rf.matchIndex[peerID]+1)

		} else if reply.Success == true { // append log successfully
			// NB: msg might be delayed !!!
			DDEBUG(LEADER_APPEN_LOG_REPLY_HANDLER,
				"[msgNo=%d], [leader=%d] append log at [peer=%d] to [index=%d]\n",
				reply.MsgNo, rf.me, peerID, curIndex)

			// update 2 index
			rf.nextIndex[peerID] = Max(curIndex+1, rf.nextIndex[peerID])
			rf.matchIndex[peerID] = Max(curIndex, rf.matchIndex[peerID])

			rf.leaderTryUpdateCommitIndex()

		} else {
			DDEBUG(LEADER_APPEN_LOG_REPLY_HANDLER,
				"WTF???\n",
			)
		}
	}
}

// caller hold the lock
// only called in `Raft.AppendLogReplyHandler()`
func (rf *Raft) leaderTryUpdateCommitIndex() {
	majority := (len(rf.peers) / 2) + 1
	tmp_matchIndex := make([]int, len(rf.matchIndex))
	copy(tmp_matchIndex, rf.matchIndex)
	majority_commit_index := findKthLargest(tmp_matchIndex, majority)
	if rf.commitIndex < majority_commit_index &&
		rf.logs[majority_commit_index].Term == rf.currentTerm { // Figure 8

		DDEBUG(FIND_MAJORITY_COMMIT_INDEX,
			"[leader=%d] update commit index, [%d->%d]\n",
			rf.me, rf.commitIndex, majority_commit_index)

		rf.commitIndex = majority_commit_index
		rf.tryCommit()
	}
}

// entry is not empty, called when client append log request.
// entry is empty, called when send heartbeat msg.
func (rf *Raft) leaderAppendLog() {

	sendEntry := func(peerID, currentTerm int) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		var args = AppendEntryArg{
			Term:              currentTerm,
			Leader:            rf.me,
			PrevLogIndex:      rf.nextIndex[peerID] - 1,
			PrevLogTerm:       rf.logs[rf.nextIndex[peerID]-1].Term,
			Entries:           make([]LogEntry, len(rf.logs)-rf.nextIndex[peerID]),
			LeaderCommitIndex: rf.commitIndex,
			MsgNo:             getMsgNo(),
		}
		copy(args.Entries, rf.logs[rf.nextIndex[peerID]:])

		DDEBUG(LEADER_APPEND_LOG,
			"[msgNo=%d], [leader=%d] in [term=%d] send log entry to [peer=%d], [next=%d], [match=%d]\n",
			args.MsgNo, rf.me, currentTerm, peerID,
			rf.nextIndex[peerID], rf.matchIndex[peerID])

		if len(args.Entries) > 1 {
			DDEBUG(LEADER_APPEND_LOG,
				"[msgNo=%d], [leader=%d] in [term=%d] send log entry to [peer=%d], [prev=(%d, %d)], [last=(%d, %d)]\n",
				args.MsgNo, rf.me, args.Term, peerID,
				args.PrevLogTerm, args.PrevLogIndex,
				args.Entries[len(args.Entries)-1].Term, args.Entries[len(args.Entries)-1].Index)
		}

		go func() {
			reply := AppendEntryReply{}
			if rf.sendAppendEntry(peerID, &args, &reply) {
				rf.AppendLogReplyHandler(peerID, &reply, args.PrevLogIndex+len(args.Entries))
			}
		}()
	}

	rf.mu.Lock()

	DDEBUG(LEADER_APPEND_LOG,
		"[leader=%d] in [term=%d] send log, [leader.lastlogs=(%d, %d)]\n",
		rf.me, rf.currentTerm, rf.logs[len(rf.logs)-1].Term, rf.logs[len(rf.logs)-1].Index)

	currentTerm := rf.currentTerm

	peer_size := len(rf.peers)

	rf.mu.Unlock()

	for i := 0; i < peer_size; i++ {
		select {
		case <-rf.shutDownCh:
		case <-rf.notLeaderCh:
			return
		default:
			if i != rf.me {
				sendEntry(i, currentTerm)
			}
		}
	}
}

func (rf *Raft) heartbeatRoutine() {
	for {
		DDEBUG(SEND_HEARTBEAT_ROUTINE,
			"[leader=%d] send heart beat in [term=%d]\n",
			rf.me, rf.currentTerm)
		select {
		// return due to shutting down
		case <-rf.shutDownCh:
			return
		// return due to turning to follower
		case <-rf.notLeaderCh:
			return
		default:
			rf.leaderAppendLog()
		}
		time.Sleep(rf.heartbeatInterval)
	}
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Leader {
		isLeader = false
	} else {
		index = len(rf.logs)
		term = rf.currentTerm
		isLeader = true
		var newEntry = LogEntry{
			Term:    term,
			Index:   index,
			Command: command,
		}
		rf.logs = append(rf.logs, newEntry)

		DDEBUG(LEADER_APPEND_LOG,
			"leader append log [leader=%d], [term=%d], [command=%v]\n",
			rf.me, rf.currentTerm, command)

		rf.nextIndex[rf.me] = index + 1
		rf.matchIndex[rf.me] = index

		rf.persist()
	}

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
	close(rf.shutDownCh)
	rf.commitCond.Signal()
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh = applyCh

	// Your initialization code here (2A, 2B, 2C).
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.leader = -1
	rf.logs = make([]LogEntry, 1) // dummy log
	rf.logs[0] = LogEntry{
		Term:    0,
		Index:   0,
		Command: 0,
	}

	rf.commitIndex = -1
	rf.lastApplied = -1
	rf.commitCond = sync.NewCond(&rf.mu)

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	// 200~400 ms
	rf.electionTimeout = time.Millisecond * time.Duration(200+rand.Intn(200))
	rf.notLeaderCh = make(chan struct{})
	rf.shutDownCh = make(chan struct{})
	rf.heartbeatInterval = time.Millisecond * 40 // small enough, not too small

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// init timer
	DDEBUG(TIMER,
		"[peer=%d] start timer\n",
		rf.me)
	rf.electionTimer = time.AfterFunc(rf.electionTimeout, func() { rf.candidateRequestVote() })

	if rf.state == Leader {
		rf.stopTimer()
		rf.electionSuccessReset()
		go rf.heartbeatRoutine()
	}

	go rf.applyEntryRoutine()

	DDEBUG(INIT, "init %d\n", rf.me)

	return rf
}
