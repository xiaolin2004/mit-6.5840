package kvsrv

import (
	"crypto/rand"
	"math/big"

	"6.5840/labrpc"
	"github.com/google/uuid"
)

type Clerk struct {
	server *labrpc.ClientEnd
	// You will have to modify this struct.
}

type Finish struct{}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(server *labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.server = server
	// You'll have to add code here.
	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.server.Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	args := GetArgs{Key: key, UUID: uuid.New()}
	reply := GetReply{}
	sig := make(chan Finish)
	go ck.callGetrepeatedky(&args, &reply, sig)
	// You will have to modify this function.
	<-sig
	return reply.Value
}

func (ck *Clerk) callGetrepeatedky(args *GetArgs, reply *GetReply, sig chan Finish) {
	ok := ck.server.Call("KVServer.Get", args, reply)
	for {
		if ok {
			sig <- Finish{}
			break
		} else {
			ok = ck.server.Call("KVServer.Get", args, reply)
		}
	}

}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.server.Call("KVServer."+op, &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) string {
	args := PutAppendArgs{Key: key, Value: value, UUID: uuid.New()}
	reply := PutAppendReply{}
	sig := make(chan Finish)
	go ck.callPutAppendrepeatedly(&args, &reply, sig, op)
	<-sig
	return reply.Value
}

func (ck *Clerk) callPutAppendrepeatedly(args *PutAppendArgs, reply *PutAppendReply, sig chan Finish, op string) {
	rpcname := "KVServer." + op
	ok := ck.server.Call(rpcname, args, reply)
	for {
		if ok {
			sig <- Finish{}
			break
		} else {
			ok = ck.server.Call(rpcname, args, reply)
		}
	}

}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}

// Append value to key's value and return that value
func (ck *Clerk) Append(key string, value string) string {
	return ck.PutAppend(key, value, "Append")
}
