package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.
// 在此进行消息传递的格式规范

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}

type RegisterArgs struct {
}

type RegisterReply struct {
	Workerindex int
}

// type HeartBeat struct {
// 	Workerindex int
// }

type TaskArgs struct {
	TaskType  TaskType
	FileName  []string
	nReduce   int
	TaskIndex int
}

type TaskReply struct {
	Workerindex int
	TaskType    TaskType
	FileName    []string
}

type TaskRequire struct {
	Workerindex int
}

type TaskReplyConfirm struct {
}
