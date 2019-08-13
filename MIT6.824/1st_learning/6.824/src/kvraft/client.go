package raftkv

import (
	"crypto/rand"
	"labrpc"
	"math/big"
	"time"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	maybeLeader int
}

func (ck *Clerk) tryNextleader() {
	ck.maybeLeader = (ck.maybeLeader + 1) % len(ck.servers)
	DDEBUG(TRY_LEADER,
		"[peer=%d]\n",
		ck.maybeLeader)
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.maybeLeader = 0
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.
	DDEBUG(GET,
		"Get(%s)\n",
		key)

	var arg = GetArgs{
		Key: key,
	}
	for {
		reply := new(GetReply)
		done := make(chan bool, 1)
		leader := ck.maybeLeader
		go func() {
			DDEBUG(GET,
				"Get(%s) at [peer=%d]\n",
				key, leader)
			ok := ck.servers[leader].Call("KVServer.Get", &arg, reply)
			done <- ok
			DDEBUG(GET,
				"Get(%s), [peer=%d] reply\n",
				key, leader)
		}()

		select {
		case <-time.After(200 * time.Millisecond): // rpc timeout: 200ms
			ck.tryNextleader()
			continue
		case ok := <-done:
			if ok && !reply.WrongLeader {
				DDEBUG(GET,
					"Get(%s)->%s ok at [peer=%d]\n",
					key, reply.Value, leader)
				return reply.Value
			}
			ck.tryNextleader()
			DDEBUG(GET,
				"Get(%s) fail at [peer=%d]\n",
				key, leader)
		}
	}
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	DDEBUG(PUT_APPEND,
		"%s [key=%s]\n",
		op, key)

	for {
		var arg = PutAppendArgs{
			Key:   key,
			Value: value,
			Op:    op,
		}
		reply := new(PutAppendReply)
		done := make(chan bool, 1)
		leader := ck.maybeLeader
		go func() {
			DDEBUG(PUT_APPEND,
				"%s [key=%s, value=%s] at [peer=%d]\n",
				op, key, value, leader)
			ok := ck.servers[leader].Call("KVServer.PutAppend", &arg, reply)
			done <- ok
			DDEBUG(PUT_APPEND,
				"%s [key=%s], [peer=%d] reply\n",
				op, key, leader)
		}()
		select {
		case <-time.After(200 * time.Millisecond): // rpc timeout: 200ms
			ck.tryNextleader()
			continue
		case ok := <-done:
			if ok && !reply.WrongLeader && reply.Err == OK {
				DDEBUG(PUT_APPEND,
					"%s [key=%s, value=%s] ok at [peer=%d]\n",
					op, key, value, leader)
				return
			}
			DDEBUG(PUT_APPEND,
				"%s [key=%s, value=%s] fail at [peer=%d]\n",
				op, key, value, leader)
			ck.tryNextleader()
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
