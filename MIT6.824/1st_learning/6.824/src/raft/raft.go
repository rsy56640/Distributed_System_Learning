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
	"labrpc"
	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "labgob"

func Max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
func Min(a, b int) int {
	if a > b {
		return b
	} else {
		return a
	}
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
	LogInfo string
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
	leader      int // -1 means donot know who is leader in current term

	// record time duration
	electionTimer     *time.Timer   // election timer
	electionTimeout   time.Duration // 200~400ms
	heartbeatInterval time.Duration // 40ms

	// volatile state on all servers.
	commitIndex int
	lastApplied int

	// volatile state on leader, reset when the server becomes leader.
	nextIndex  []int
	matchIndex []int

	// sync util
	notLeaderCh chan struct{}
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
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
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
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
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
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	CurrentTerm int // used when not vote, return self term if candidate is out of date.
	VoteGranted bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).

	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

	// update self if necessary
	// no need to reset timer
	if rf.currentTerm < args.CandidateTerm {
		DPrintf("[candidate=%d] RequestVoteArgs [peer=%d], self is out, new [term=%d]\n",
			args.CandidateID, rf.me, args.CandidateTerm)
		rf.turnToFollower(args.CandidateTerm)
	}

	// reject if candidate is out of date
	if rf.currentTerm > args.CandidateTerm {
		DPrintf("[candidate=%d] RequestVoteArgs [peer=%d], candidate is out, new [term=%d]\n",
			args.CandidateID, rf.me, rf.currentTerm)
		reply.CurrentTerm = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// not voted yet or msg lost/retransmit
	if rf.votedFor == -1 || rf.votedFor == args.CandidateID {
		// compare the term and index
		vote := args.LastLogTerm > rf.logs[len(rf.logs)-1].Term
		if args.LastLogTerm == rf.logs[len(rf.logs)-1].Term {
			vote = args.LastLogIndex >= rf.logs[len(rf.logs)-1].Index
		}
		if vote {
			DPrintf("[candidate=%d] RequestVoteArgs [peer=%d], get vote !!!\n",
				args.CandidateID, rf.me)
			reply.VoteGranted = true
			// update self, reset timer
			rf.state = Follower
			rf.currentTerm = args.CandidateTerm
			rf.votedFor = args.CandidateID
			rf.leader = args.CandidateID
			rf.resetTimer()
		} else {
			DPrintf("[candidate=%d] RequestVoteArgs [peer=%d], reject due to log safety !!!\n",
				args.CandidateID, rf.me)
			reply.VoteGranted = false
			reply.CurrentTerm = -1
			return
		}
	} else { // have voted for other, so reject
		DPrintf("[candidate=%d] RequestVoteArgs [peer=%d], reject since have voted for [peer=%d] !!!\n",
			args.CandidateID, rf.me, rf.votedFor)
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
		if i == rf.me {
			rf.matchIndex[i] = length - 1
		}
	}
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
	}
	DPrintf("[candidate=%d] begin canvass, [term=%d]\n", rf.me, rf.currentTerm)
	rf.mu.Unlock()

	vote := 1

	// maybe network delay when reply
	replyHandler := func(reply *RequestVoteReply, peerID int) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		// update routine
		if reply.VoteGranted == false && reply.CurrentTerm > rf.currentTerm {
			DPrintf("[candidate=%d] turn to follower, new [term=%d]\n",
				rf.me, reply.CurrentTerm)
			rf.turnToFollower(reply.CurrentTerm)
		}

		if rf.state == Candidate { // else discard
			if reply.VoteGranted == true { // vote success
				vote++
				DPrintf("[candidate=%d] has [vote=%d]\n",
					rf.me, vote)
				if vote*2 > peer_size {
					// no need to reset timer when become leader
					DPrintf("[candidate=%d] becomes leader\n", rf.me)
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
			replyHandler(&voteReply, peerID)
		}(i)
	}
}

func (rf *Raft) resetTimer() {
	rf.electionTimer.Stop()
	rf.electionTimer = time.AfterFunc(rf.electionTimeout, func() { rf.candidateRequestVote() })
}

func (rf *Raft) stopTimer() {
	rf.electionTimer.Stop()
}

type AppendEntryArg struct {
	Term              int
	Leader            int
	PrevLogIndex      int
	PrevLogTerm       int
	Entries           []LogEntry
	LeaderCommitIndex int
}

type AppendEntryReply struct {
	// used when fail, return
	//		-1 				if log is dismatched, or
	//		self.term 		if the msg is delayed.
	Term int

	Success bool
}

func (rf *Raft) AppendEntry(args *AppendEntryArg, reply *AppendEntryReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

	// update self
	if args.Term > rf.currentTerm {
		DPrintf("append log RPC [leader=%d], [peer=%d], self is out, new [term=%d]\n",
			args.Leader, rf.me, args.Term)
		rf.turnToFollower(args.Term)
	}

	if args.Term < rf.currentTerm {
		// network delay
		// return failure and current term to notice the leader,
		// leader will find the term is same or less, then discard.
		DPrintf("append log RPC [leader=%d], [peer=%d], msg is delayed, current [term=%d]\n",
			args.Leader, rf.me, args.Term)
		reply.Success = false
		reply.Term = rf.currentTerm
	} else { // args.term == rf.currentTerm

		// the current leader's message, regardless of network delay.
		rf.resetTimer()

		// attempt to append log entries
		// first check previous term and index
		if args.PrevLogIndex == rf.logs[len(rf.logs)-1].Index &&
			args.PrevLogTerm == rf.logs[len(rf.logs)-1].Term {
			// update log
			update_size := len(rf.logs) - 1 - args.PrevLogIndex
			for i := 0; i < update_size; i++ {
				rf.logs[i+args.PrevLogIndex+1] = args.Entries[i]
			}
			// append log
			append_size := len(args.Entries) - update_size
			for i := update_size; i < update_size+append_size; i++ {
				rf.logs = append(rf.logs, args.Entries[i])
			}

			reply.Success = true

		} else { // check fail
			reply.Term = -1
			reply.Success = false
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
	if reply.Success == false && reply.Term > rf.currentTerm {
		DPrintf("append log at [peer=%d] failed, term is out, new [term=%d]\n",
			peerID, reply.Term)
		rf.turnToFollower(reply.Term)
	}

	// if self is still a leader
	if rf.state == Leader {
		// NB: msg might be delayed !!!
		if reply.Success == false && reply.Term == -1 { // log dismatched, but msg maybe delayed.
			DPrintf("log dismatched at [peer=%d]\n", peerID)
			rf.nextIndex[peerID] = Max(rf.nextIndex[peerID]-1, rf.matchIndex[peerID])
		} else { // append log successfully
			// NB: msg might be delayed !!!
			DPrintf("append log at [peer=%d]\n", peerID)
			// update 2 index
			rf.nextIndex[peerID] = Max(curIndex+1, rf.nextIndex[peerID])
			rf.matchIndex[peerID] = Max(curIndex, rf.matchIndex[peerID])
		}
	}
}

// entry is not empty, called when client append log request.
// entry is empty, called when send heartbeat msg.
func (rf *Raft) leaderAppendLog() {
	sendEntry := func(peerID int) {
		rf.mu.Lock()
		defer rf.mu.Unlock()
		var args = AppendEntryArg{
			Term:              rf.currentTerm,
			Leader:            rf.me,
			PrevLogIndex:      rf.nextIndex[peerID] - 1,
			PrevLogTerm:       rf.logs[rf.nextIndex[peerID]-1].Term,
			Entries:           rf.logs[rf.nextIndex[peerID]:],
			LeaderCommitIndex: rf.commitIndex,
		}
		go func() {
			reply := AppendEntryReply{}
			if rf.sendAppendEntry(peerID, &args, &reply) {
				rf.AppendLogReplyHandler(peerID, &reply, args.PrevLogIndex+len(args.Entries))
			}
		}()
	}

	peer_size := len(rf.peers)
	for i := 0; i < peer_size; i++ {
		if i != rf.me {
			//DPrintf("[leader=%d] in [term=%d] send log entry to [peer=%d]\n",
			//	rf.me, rf.currentTerm, i)
			sendEntry(i)
		}
	}
}

func (rf *Raft) heartbeatRoutine() {
	for {
		DPrintf("[leader=%d] send heart beat in [term=%d]\n",
			rf.me, rf.currentTerm)
		select {
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

	// Your initialization code here (2A, 2B, 2C).
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.leader = -1
	rf.logs = make([]LogEntry, 1) // dummy log
	rf.logs[0] = LogEntry{
		Term:    0,
		Index:   0,
		LogInfo: "",
	}

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	// 200~400 ms
	rf.electionTimeout = time.Millisecond * time.Duration(200+rand.Intn(200))
	rf.notLeaderCh = make(chan struct{})
	rf.heartbeatInterval = time.Millisecond * 40 // small enough, not too small
	// init timer
	rf.electionTimer = time.AfterFunc(rf.electionTimeout, func() { rf.candidateRequestVote() })

	DPrintf("init %d\n", rf.me)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
