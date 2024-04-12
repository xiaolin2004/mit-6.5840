package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

type KWorker struct {
	mapf        func(string, string) []KeyValue
	reducef     func(string, []string) string
	Workerindex int
	state       WorkerState
}

func MakeWorker(mapf func(string, string) []KeyValue, reducef func(string, []string) string) *KWorker {
	w := KWorker{}
	w.mapf = mapf
	w.reducef = reducef
	w.state = Workeridle
	return &w
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	w := MakeWorker(mapf, reducef)
	w.RegisterSelf()
	w.Work()
}

func (w *KWorker) RegisterSelf() {
	args := RegisterArgs{}
	reply := RegisterReply{}
	ok := call("Coordinator.Register", &args, &reply)
	if ok {
		fmt.Printf("Coordinator is available, register succuss, get id %v.\n", reply.Workerindex)
		w.Workerindex = reply.Workerindex
	} else {
		fmt.Printf("Coordinator is down")
	}
}

func (w *KWorker) ApplyTask() TaskArgs {
	args := TaskRequire{
		Workerindex: w.Workerindex,
	}
	reply := TaskArgs{}
	call("Coordinator.GetTask", &args, &reply)
	return reply
}

func (w *KWorker) ReplyTask(FileName []string, TaskType TaskType) {
	args := TaskReply{}
	reply := TaskReplyConfirm{}
	args.FileName = FileName
	args.Workerindex = w.Workerindex
	args.TaskType = TaskType
	call("Coordinator.FinishWork", &args, &reply)
}

func (w *KWorker) Work() {
	for {
		Task := w.ApplyTask()
		if Task.TaskType != Complete {
			switch Task.TaskType {
			case Map:
				w.DoMap(Task.FileName, Task.NReduce, Task.TaskIndex)
			case Reduce:
				w.DoReduce(Task.FileName, Task.TaskIndex)
			case Complete:
				time.Sleep(10 * time.Second)
			}
		}
	}
}

func (w *KWorker) DoMap(FileName []string, nReduce int, TaskIndex int) {
	filename := FileName[0]
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()
	kva := w.mapf(filename, string(content))
	tmp := splitSlice(kva, nReduce)
	retFileName := make([]string, 0)
	for index, kva := range tmp {
		filename = fmt.Sprintf("tmpMapInter-%v-%v", TaskIndex, index)
		file, err = os.Create(filename)
		if err != nil {
			log.Fatalf("cannot creat %v", filename)
		}
		enc := json.NewEncoder(file)
		for _, kv := range kva {
			enc.Encode(kv)
		}
		retFileName = append(retFileName, filename)
	}
	w.ReplyTask(retFileName, Map)
}

func (w *KWorker) DoReduce(FileName []string, TaskIndex int) {
	kva := make([]KeyValue, 0)
	fmt.Printf("get reduce task:{File:%+v , index:%v}\n", FileName, TaskIndex)
	for _, filename := range FileName {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("the kv decoded are %+v\n", kv)
			kva = append(kva, kv)
		}

	}
	sort.Sort(ByKey(kva))
	ret := []KeyValue{}
	i := 0
	for i < len(kva) {
		j := i + 1
		for j < len(kva) && kva[j].Key == kva[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, kva[k].Value)
		}
		output := w.reducef(kva[i].Key, values)
		ret = append(ret, KeyValue{kva[i].Key, output})
		i = j
	}
	filename := fmt.Sprintf("tmpmr-out-%v", TaskIndex)
	ofile, err := os.Create(filename)
	if err != nil {
		log.Fatalf("cannot creat %v", filename)
	}
	for _, kv := range ret {
		fmt.Fprintf(ofile, "%v %v\n", kv.Key, kv.Value)
	}
	w.ReplyTask([]string{filename}, Reduce)
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
