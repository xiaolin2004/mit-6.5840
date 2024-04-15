package kvsrv

import (
	"log"
	"sync"

	"github.com/google/uuid"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Operation struct {
	Optype Optype
	Key    string
	Value  string
	Opid   uuid.UUID
}

type Optype int

const (
	OpGet    = 100
	OpPut    = 101
	OpAppend = 102
)

type KVServer struct {
	mu      sync.Mutex
	kvStore map[string]string
	OpCache *LruCache
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	task := Operation{Optype: OpGet, Opid: args.UUID, Key: args.Key}
	kv.Op(&task)
	reply.Value = task.Value
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	task := Operation{Optype: OpPut, Opid: args.UUID, Key: args.Key, Value: args.Value}
	kv.Op(&task)
	reply.Value = ""
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	task := Operation{Optype: OpAppend, Opid: args.UUID, Key: args.Key, Value: args.Value}
	kv.Op(&task)
	reply.Value = task.Value
}

func StartKVServer() *KVServer {
	kv := new(KVServer)
	kv.kvStore = make(map[string]string)
	kv.mu = sync.Mutex{}
	kv.OpCache = newLruCache(30)
	return kv
}

func (kv *KVServer) Op(task *Operation) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if kv.OpCache.Contain(task.Opid) {
		oldop, _ := kv.OpCache.Get(task.Opid)
		task.Value = oldop.Value
	} else {
		switch task.Optype {
		case OpGet:
			task.Value = kv.kvStore[task.Key]
		case OpAppend:
			old, ok := kv.kvStore[task.Key]
			if !ok {
				old = ""
			}
			kv.kvStore[task.Key] += task.Value
			task.Value = old
		case OpPut:
			kv.kvStore[task.Key] = task.Value
		}
		kv.OpCache.Add(*task)
	}

}
