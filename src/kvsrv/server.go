package kvsrv

import (
	"fmt"
	"log"

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
	kvStore map[string]string
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
	Manager Manager
	// Your definitions here.
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	task := Operation{Optype: OpGet, Opid: args.UUID, Args: args, Reply: reply, Empty: false}
	kv.Manager.PushOp(task)
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	task := Operation{Optype: OpPut, Opid: args.UUID, Args: args, Reply: reply, Empty: false}
	kv.Manager.PushOp(task)
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	task := Operation{Optype: OpAppend, Opid: args.UUID, Args: args, Reply: reply, Empty: false}
	kv.Manager.PushOp(task)
}

func StartKVServer() *KVServer {
	kv := new(KVServer)
	kv.Manager = Manager{kvStore: make(map[string]string), OpQueue: *NewSliceQueue(10), OpCache: NewCache(10)}
	go func() {
		for {
			kv.Manager.Op()
		}
	}()
	// go func ()  {
	// 	for{
	// 		fmt.Printf("%v\n",kv.Manager.kvStore)
	// 		time.Sleep(1*time.Second)
	// 	}
	// }()
	return kv
}

func (m *Manager) PushOp(op Operation) {
	m.OpQueue.Enqueue(op)
}

func (m *Manager) Op() {
	if m.OpQueue.isEmpty() {
		return
	}
	task := m.OpQueue.Dequeue()
	if m.OpCache.Contains(task.Opid) {
		//如果是修改操作，直接返回成功，不执行
		//如果是读取操作，直接查询返回
		switch task.Optype {
		case OpGet:
			task.Reply.(*GetReply).Value = (m.kvStore)[task.Args.(*GetArgs).Key]
		case OpPut:
		case OpAppend:
			task.Reply.(*PutAppendReply).Value = "" //需要参看通讯规范
		}
	} else {
		switch task.Optype {
		case OpGet:
			fmt.Printf("get key:%v\n", task.Args.(*GetArgs).Key)
			task.Reply.(*GetReply).Value = (m.kvStore)[task.Args.(*GetArgs).Key]
		case OpPut:
			fmt.Printf("put op :put k:%v v:%v\n", (task.Args.(*PutAppendArgs).Key), (task.Args.(*PutAppendArgs).Value))
			(m.kvStore)[task.Args.(*PutAppendArgs).Key] = task.Args.(*PutAppendArgs).Value
			// fmt.Printf("kvstore detail:%+v\n",m.kvStore)
			task.Reply.(*PutAppendReply).Value = ""
		case OpAppend:
			fmt.Printf("append op :append k:%v v:%v\n", (task.Args.(*PutAppendArgs).Key), (task.Args.(*PutAppendArgs).Value))
			old := (m.kvStore)[task.Args.(*PutAppendArgs).Key]
			(m.kvStore)[task.Args.(*PutAppendArgs).Key] += task.Args.(*PutAppendArgs).Value
			task.Reply.(*PutAppendReply).Value = old
		}
		m.OpCache.Add(task.Opid)
	}
}
