package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"
)

const TimeOut = 10

type TaskStates int

const (
	Taskidle       = 100
	Taskinprogress = 101
	Taskcomplete   = 102
)

type WorkerState int

const (
	Workeridle = 200
	Workerbusy = 201
	Workerdown = 202
)

type WorkerList []WorkerState

type TaskList struct {
	Map    SliceQueue
	Reduce SliceQueue
}

func (c *Coordinator) Dequeue(TaskType TaskType) []string {
	switch TaskType {
	case Map:
		return c.tasklist.Map.Dequeue()
	case Reduce:
		return c.tasklist.Reduce.Dequeue()
	}
	return make([]string, 0)
}

func (c *Coordinator) Enqueue(TaskType TaskType, FileName []string) {
	switch TaskType {
	case Map:
		c.tasklist.Map.Enqueue(FileName)
	case Reduce:
		c.tasklist.Reduce.Enqueue(FileName)
	}
}

type TaskType int

const (
	Map      = 300
	Reduce   = 301
	Complete = 302
)

type FinishMsg struct{}

type Task struct {
	FileName  []string
	TaskIndex int
}

type Coordinator struct {
	state      TaskType
	nReduce    int
	tasklist   TaskList
	workerList WorkerList
	TimeOut    []chan FinishMsg
	//这两个Index是没有考虑过worker下线的，需要重新设计
	MapIndex    int
	ReduceIndex int
}

func (c *Coordinator) Register(args *RegisterArgs, reply *RegisterReply) error {
	c.workerList = append(c.workerList, Workeridle)
	c.TimeOut = append(c.TimeOut, make(chan FinishMsg))
	reply.Workerindex = int(len(c.workerList))
	return nil
}

func (c *Coordinator) GetTask(args *TaskRequire, reply *TaskArgs) error {

	// 写入reply
	reply.TaskType = c.state
	tmpFileName := c.Dequeue(c.state)
	reply.FileName = tmpFileName
	reply.nReduce = c.nReduce
	if c.state == Map {
		reply.TaskIndex = c.MapIndex
		c.MapIndex++
	} else if c.state == Reduce {
		reply.TaskIndex = c.ReduceIndex
		c.ReduceIndex++
	}
	// 为任务设置超时
	go func() {
		select {
		case <-c.TimeOut[args.Workerindex]:
			c.workerList[args.Workerindex] = Workeridle
		case <-time.After(TimeOut * time.Second):
			c.workerList[args.Workerindex] = Workerdown
			c.Enqueue(c.state, tmpFileName)
		}
	}()
	return nil
}

// func (c *Coordinator) HeartBeat(args *HeartBeat, reply interface{}) error {
// 	//针对超时机制修改
// 	return nil
// }

func (c *Coordinator) FinishWork(args *TaskReply, reply interface{}) error {
	//向对应计时器发送消息
	c.TimeOut[args.Workerindex] <- FinishMsg{}
	//读取任务类型，如果是map，首先将文件统一改名，然后打包丢进reduce队列
	var newname string
	if args.TaskType == Map {
		FileName := []string{}
		for _, filename := range args.FileName {
			newname = filename[3:]
			err := os.Rename(filename, newname)
			if err != nil {
				log.Fatalf("cannot commit %v", filename)
			}
			FileName = append(FileName, filename)
		}
		c.Enqueue(Reduce, FileName)
	} else if args.TaskType == Reduce { //如果是reduce，统一改名
		if args.TaskType == Map {
			for _, filename := range args.FileName {
				newname = filename[3:]
				err := os.Rename(filename, newname)
				if err != nil {
					log.Fatalf("cannot commit %v", filename)
				}
			}
		}
	}

	//修改worker状态
	//在超时机制的实现已经修改
	return nil

}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	if c.state == Map {
		return false
	}
	for w := range c.workerList[:] {
		if w == Workerbusy {
			return false
		}
	}
	t := c.tasklist.Reduce.Dequeue()
	if t != nil {
		return false
	}
	ret := true

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	filenum := len(files)
	c := Coordinator{}
	c.nReduce = nReduce
	c.state = Map
	c.workerList = make(WorkerList, 0)
	c.tasklist = TaskList{Map: *NewSliceQueue(filenum), Reduce: *NewSliceQueue(filenum * nReduce)}
	for _, f := range files {
		c.tasklist.Map.Enqueue([]string{f})
	}
	c.TimeOut = make([]chan FinishMsg, 0)
	c.server()
	return &c
}
