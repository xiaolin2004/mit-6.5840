package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
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

type WorkerList struct {
	list []WorkerState
	mu   sync.Mutex
}

func (l *WorkerList) noBusy() bool {
	nobusy := true
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, state := range l.list {
		if state == Workerbusy {
			return false
		}
	}
	return nobusy
}

type TaskList struct {
	Map    SliceQueue
	Reduce SliceQueue
}

func (c *Coordinator) DelegateIndex(task *Task, TaskType TaskType) {
	if TaskType == Map && task.TaskIndex < 0 {
		c.LockMapIndex.Increment()
		task.TaskIndex = c.LockMapIndex.Read()
	}
	if TaskType == Reduce && task.TaskIndex < 0 {
		c.LockReduceIndex.Increment()
		task.TaskIndex = c.LockReduceIndex.Read()
	}
}
func (c *Coordinator) Dequeue(TaskType TaskType) Task {
	switch TaskType {
	case Map:
		return c.tasklist.Map.Dequeue()
	case Reduce:
		return c.tasklist.Reduce.Dequeue()
	}
	return Task{}
}

func (c *Coordinator) Enqueue(TaskType TaskType, Task Task) {
	c.DelegateIndex(&Task, TaskType)
	switch TaskType {
	case Map:
		c.tasklist.Map.Enqueue(Task)
	case Reduce:
		c.tasklist.Reduce.Enqueue(Task)
	}
}

type TaskType int

const (
	Map      = 300
	Reduce   = 301
	Complete = 302
	Wait     = 303
)

type FinishMsg struct{}

type Task struct {
	FileName  []string
	TaskIndex int
}

type Coordinator struct {
	state           TaskType
	nReduce         int
	tasklist        TaskList
	workerList      WorkerList
	TimeOut         []chan FinishMsg
	LockMapIndex    LockMIndex
	LockReduceIndex LockMIndex
}

func (c *Coordinator) Register(args *RegisterArgs, reply *RegisterReply) error {
	c.workerList.mu.Lock()
	defer c.workerList.mu.Unlock()
	c.workerList.list = append(c.workerList.list, Workeridle)
	c.TimeOut = append(c.TimeOut, make(chan FinishMsg))
	reply.Workerindex = int(len(c.workerList.list) - 1)
	return nil
}

func (c *Coordinator) GetTask(args *TaskRequire, reply *TaskArgs) error {
	// 写入reply
	reply.TaskType = c.state
	tmpTask := c.Dequeue(c.state)
	if len(tmpTask.FileName) == 0 {
		reply.TaskType = Wait
		return nil
	}
	reply.FileName = tmpTask.FileName
	reply.NReduce = c.nReduce
	reply.TaskIndex = tmpTask.TaskIndex
	c.workerList.mu.Lock()
	c.workerList.list[args.Workerindex] = Workerbusy
	c.workerList.mu.Unlock()
	// fmt.Printf("send task:%+v\n", reply)
	// 为任务设置超时
	go func() {
		select {
		case <-c.TimeOut[args.Workerindex]:
			// fmt.Printf("get signal, worker %v finish\n", args.Workerindex)
			c.workerList.mu.Lock()
			c.workerList.list[args.Workerindex] = Workeridle
			c.workerList.mu.Unlock()
		case <-time.After(TimeOut * time.Second):
			// fmt.Printf("Timeout, worker %v down\n", args.Workerindex)
			c.workerList.mu.Lock()
			c.workerList.list[args.Workerindex] = Workerdown
			c.workerList.mu.Unlock()
			c.Enqueue(c.state, tmpTask)
		}
	}()
	return nil
}

func (c *Coordinator) FinishWork(args *TaskReply, reply *TaskReplyConfirm) error {
	// fmt.Printf("get reply:%+v\n", args)
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
			FileName = append(FileName, newname)
		}
		for _, filename := range FileName {
			// lastIndex := len(filename) - 1
			// index, err := strconv.Atoi(filename[lastIndex-1 : lastIndex])
			r, _ := utf8.DecodeLastRuneInString(filename)
			lastChar := string(r)
			index, err := strconv.Atoi(lastChar)
			if err != nil {
				log.Fatalf("cannot cast the %v to int", lastChar)
			}
			c.tasklist.Reduce.mu.Lock()
			c.tasklist.Reduce.data[index].FileName = append(c.tasklist.Reduce.data[index].FileName, filename)
			c.tasklist.Reduce.mu.Unlock()
		}
	} else if args.TaskType == Reduce { //如果是reduce，统一改名
		for _, filename := range args.FileName {
			newname = filename[3:]
			err := os.Rename(filename, newname)
			if err != nil {
				log.Fatalf("cannot commit %v", filename)
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
	// go func() {
	// 	for {
	// 		fmt.Printf("workerList: %+v\n", c.workerList.list)
	// 		time.Sleep(1 * time.Second)
	// 	}
	// }()
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	if c.state == Complete {
		return true
	} else {
		return false
	}
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	filenum := len(files)
	c := Coordinator{}
	c.nReduce = nReduce
	c.state = Map
	c.workerList = WorkerList{list: []WorkerState{}, mu: sync.Mutex{}}
	c.tasklist = TaskList{Map: *NewSliceQueue(filenum), Reduce: *NewSliceQueue(nReduce)}
	for index, f := range files {
		c.tasklist.Map.Enqueue(Task{FileName: []string{f}, TaskIndex: index})
	}
	i := 0
	for {
		if i >= nReduce {
			break
		}
		c.Enqueue(Reduce, Task{FileName: []string{}, TaskIndex: -1})
		i++
	}
	c.TimeOut = make([]chan FinishMsg, 0)
	go func() {
		for {
			c.StateTransit()
			time.Sleep(10 * time.Microsecond)
		}
	}()
	c.server()
	return &c
}

func (c *Coordinator) StateTransit() {
	switch c.state {
	case Map:
		if c.tasklist.Map.isEmpty() {
			if c.tasklist.Map.isEmpty() && c.workerList.noBusy() {
				c.state = Reduce
			}
		}
	case Reduce:
		if c.tasklist.Reduce.isEmpty() {
			if c.tasklist.Reduce.isEmpty() && c.workerList.noBusy() {
				c.state = Complete
			}
		}
	case Complete:
		return
	}

}
