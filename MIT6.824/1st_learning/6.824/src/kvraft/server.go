package raftkv

import (
	"labgob"
	"labrpc"
	"log"
	"raft"
	"sync"
)

const (
	// used in the test file
	MARK        = false
	SIMPLE_TEST = false

	TRY_LEADER = false

	GET        = true
	PUT_APPEND = true

	SERVER_GET        = true
	SERVER_PUT_APPEND = true
)

func DDEBUG(debug_state bool, format string, a ...interface{}) (n int, err error) {
	if debug_state == true {
		return DPrintf(format, a...)
	}
	return 0, nil
}

// Debugging
const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.

	persist *raft.Persister
	db      map[string]string

	// Index returned from `rf.Start(cmd)`, check the cmd at index when apply db
	promise map[int]chan PutAppendArgs

	// shutdown chan
	shutDownCh chan struct{}
}

func (kv *KVServer) GetState() (int, bool) {
	return kv.rf.GetState()
}

func (kv *KVServer) isLeader() bool {
	_, isLeader := kv.rf.GetState()
	return isLeader
}

func (kv *KVServer) applyRoutine() {
	for {
		var args PutAppendArgs
		select {
		case <-kv.shutDownCh:
			return
		case applyMsg := <-kv.applyCh:
			args = applyMsg.Command.(PutAppendArgs)

			kv.mu.Lock()
			defer kv.mu.Unlock()

			// notify the blocking `PutAppend()` to check and return
			ch, present := kv.promise[applyMsg.CommandIndex]
			if present {
				ch <- args
				delete(kv.promise, applyMsg.CommandIndex)
			}

			if args.Op == "Put" {
				kv.db[args.Key] = args.Value
			} else {
				value, present := kv.db[args.Key]
				if present {
					kv.db[args.Key] = value + args.Value
				} else {
					kv.db[args.Key] = args.Value
				}
			}
		}
	}
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	if kv.isLeader() {
		reply.WrongLeader = false
		reply.Err = ""
		value, present := kv.db[args.Key]
		if present {
			reply.Value = value
		} else {
			reply.Value = ""
		}
		DDEBUG(SERVER_GET,
			"[peer=%d] is leader, Get(%s)->%s\n",
			kv.me, args.Key, reply.Value)
	} else {
		reply.WrongLeader = true
		reply.Err = ""
		DDEBUG(SERVER_GET,
			"[peer=%d] is not leader, Get(%s) fail\n",
			kv.me, args.Key)
	}
}

func cmd_equal(arg1, arg2 *PutAppendArgs) bool {
	if arg1.Key != arg2.Key {
		return false
	}
	if arg1.Value != arg2.Value {
		return false
	}
	if arg1.Op != arg2.Op {
		return false
	}
	return true
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	index, _, isLeader := kv.rf.Start(args)
	if !isLeader {
		reply.WrongLeader = true
		DDEBUG(SERVER_PUT_APPEND,
			"[peer=%d] is not leader, %s[%s, %s] fail\n",
			kv.me, args.Op, args.Key, args.Value)

		return
	}
	DDEBUG(SERVER_PUT_APPEND,
		"[peer=%d] is leader, %s[%s, %s] will might be ok at [index=%d]\n",
		kv.me, args.Op, args.Key, args.Value, index)

	kv.mu.Lock()
	kv.promise[index] = make(chan PutAppendArgs)
	ch, _ := kv.promise[index]
	kv.mu.Unlock()

	// wait until commit such index
	select {
	case <-kv.shutDownCh:
		return

	case applyArg := <-ch:
		if cmd_equal(args, &applyArg) {
			reply.WrongLeader = false
			reply.Err = OK
			DDEBUG(SERVER_PUT_APPEND,
				"[peer=%d] is leader, %s[%s, %s] ok at [index=%d]\n",
				kv.me, args.Op, args.Key, args.Value, index)
		} else {
			reply.WrongLeader = true
			DDEBUG(SERVER_PUT_APPEND,
				"[peer=%d] is leader, %s[%s, %s] has not been commit\n",
				kv.me, args.Op, args.Key, args.Value)
		}
	}

}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *KVServer) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.
	kv.persist = persister
	kv.applyCh = make(chan raft.ApplyMsg)
	kv.db = make(map[string]string)

	// Index returned from `rf.Start(cmd)`, check the cmd at index when apply db
	kv.promise = make(map[int]chan PutAppendArgs)

	// shutdown chan
	kv.shutDownCh = make(chan struct{})

	// You may need initialization code here.

	// apply log from raft
	go func() {
		kv.applyRoutine()
	}()

	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	return kv
}
