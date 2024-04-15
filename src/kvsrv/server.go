package kvsrv

import (
	"log"
	"time"

	"github.com/google/uuid"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Manager struct {
	kvStore *map[string]string
	OpQueue SliceQueue
	OpCache *Cache
}

type Args interface{}
type Reply interface{}

type Operation struct {
	Empty  bool
	Optype Optype
	Args   Args
	Reply  Reply
	Opid   uuid.UUID
}

type Optype int

const (
	OpGet    = 100
	OpPut    = 101
	OpAppend = 102
)

type KVServer struct {
	kvStore map[string]string
	Manager Manager
	// Your definitions here.
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	task := Operation{Optype: OpGet, Opid: args.UUID, Args: args, Reply: reply,Empty: false}
	kv.Manager.PushOp(task)
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	task := Operation{Optype: OpPut, Opid: args.UUID, Args: args, Reply: reply,Empty: false}
	kv.Manager.PushOp(task)
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	task := Operation{Optype: OpAppend, Opid: args.UUID, Args: args, Reply: reply,Empty: false}
	kv.Manager.PushOp(task)
}

func StartKVServer() *KVServer {
	kv := new(KVServer)
	kv.kvStore = make(map[string]string)
	kv.Manager = Manager{OpQueue: *NewSliceQueue(10), OpCache: NewCache(10), kvStore: &kv.kvStore}
	go func ()  {
		for{
			kv.Manager.Op()
			time.Sleep(1000*time.Millisecond)
		}	
	}()
	return kv
}

func (m *Manager) PushOp(op Operation) {
	m.OpQueue.Enqueue(op)
}

func (m *Manager) Op() {
	if m.OpQueue.isEmpty(){
		return 
	}
	task := m.OpQueue.Dequeue()
	if m.OpCache.Contains(task.Opid) {
		//如果是修改操作，直接返回成功，不执行
		//如果是读取操作，直接查询返回
		switch task.Optype {
		case OpGet:
			task.Reply.(*GetReply).Value = (*m.kvStore)[task.Args.(*GetArgs).Key]
		case OpPut:
		case OpAppend:
			task.Reply.(*PutAppendReply).Value = "" //需要参看通讯规范
		}
	} else {
		switch task.Optype {
		case OpGet:
			task.Reply.(*GetReply).Value = (*m.kvStore)[task.Args.(*GetArgs).Key]
		case OpPut:
			(*m.kvStore)[task.Args.(*PutAppendArgs).Key] = task.Args.(*PutAppendArgs).Value
			task.Reply.(*PutAppendReply).Value = ""
		case OpAppend:
			old :=(*m.kvStore)[task.Args.(*PutAppendArgs).Key]
			(*m.kvStore)[task.Args.(*PutAppendArgs).Key] += task.Args.(*PutAppendArgs).Value
			task.Reply.(*PutAppendReply).Value = old
		}
		m.OpCache.Add(task.Opid)
	}
}
