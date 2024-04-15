package kvsrv

import "github.com/google/uuid"

// Put or Append
type PutAppendArgs struct {
	Args
	Key   string
	Value string
	UUID  uuid.UUID
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Reply
	Value string
}

type GetArgs struct {
	Args
	Key  string
	UUID uuid.UUID
	// You'll have to add definitions here.
}

type GetReply struct {
	Reply
	Value string
}
