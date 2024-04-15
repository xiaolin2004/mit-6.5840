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

var OpPool = sync.Pool{}

type KVServer struct {
	mu      sync.RWMutex
	kvStore map[string]string
	OpCache *LruCache
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	task := OpPool.Get().(*Operation)
	task.Optype = OpGet
	task.Opid = args.UUID
	task.Key = args.Key
	kv.Op(task)
	reply.Value = task.Value
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	task := OpPool.Get().(*Operation)
    task.Optype = OpPut
    task.Opid = args.UUID
    task.Key = args.Key
	task.Value=args.Value
	kv.Op(task)
	reply.Value = ""
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	task := OpPool.Get().(*Operation)
    task.Optype = OpAppend
    task.Opid = args.UUID
    task.Key = args.Key
    task.Value=args.Value
	kv.Op(task)
	reply.Value = task.Value
}

func StartKVServer() *KVServer {
	kv := new(KVServer)
	kv.kvStore = make(map[string]string)
	kv.mu = sync.RWMutex{}
	kv.OpCache = newLruCache(100)
	OpPool = sync.Pool{New: func() interface{} {
		return new(Operation)
	}}
	return kv
}

func (kv *KVServer) Op(task *Operation) {
	kv.mu.RLock()
	if kv.OpCache.Contain(task.Opid) {
		oldop, _ := kv.OpCache.Get(task.Opid)
		kv.mu.RUnlock()
		task.Value = oldop.Value
	} else {
		switch task.Optype {
		case OpGet:
			task.Value = kv.kvStore[task.Key]
			kv.mu.RUnlock()
		case OpAppend:
			old, ok := kv.kvStore[task.Key]
			if !ok {
				old = ""
			}
			kv.mu.RUnlock()
			kv.mu.Lock()
			kv.kvStore[task.Key] += task.Value
			task.Value = old
			kv.OpCache.Add(*task)
			kv.mu.Unlock()
		case OpPut:
			kv.mu.RUnlock()
			kv.mu.Lock()
			kv.kvStore[task.Key] = task.Value
			kv.OpCache.Add(*task)
			kv.mu.Unlock()
		}

	}

}
