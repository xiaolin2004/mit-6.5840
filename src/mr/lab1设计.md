# Master
Master数据结构
- 对于每一个任何类型的任务，都要记录他的状态(idle, in-progress, or completed)以及其对应的workerid
  - TaskList设计
    - 分别设置Map任务队列和Reduce任务队列,当且仅当所有Map任务完成，才开始Reduce任务的发放
    - Map任务与源文件名挂钩
    - Reduce任务与json文件名挂钩
    - 任务编号分配由Master根据文件输入次序生成，map和reduce分开
- 追踪每一个在线worker的状态（idle，busy，down）

Master行为
- Worker状态检测
  - 阶段性ping每一个worker，确保其在线
  - 如果在超时时间内master没有收到心跳包，将Worker记为down，如果worker已经被分配任务，则任务重新回到idle状态重新分配

- 任务发放
  - ~~遍历WorkerList，找到第一个idle的Worker，将其设置为busy，然后使用rpc派发任务~~(无法实现，因为master不能主动向worker发送消息)
  - ~~使用RPC将需要执行的任务类型与任务文件发送给worker（不需要确认包，设置超时，如果时间超过，认为worker下线或者掉队，直接分配给另一个空闲的worker），将任务状态设置为in-progress，然后将任务与workerid对应，并等待worker发回完成消息~~
  - 注册成功后，worker会发来任务请求，请求中携带自己的index，将workerlist中对应的worker设置为busy，从TaskList中取出任务，生成任务包（包含文件名和任务类型）发回
  
- 退出机制
  - main/mrcoordinator.go 期望 mr/coordinator.go 实现一个 Done() 方法，该方法在 MapReduce 作业完全完成时返回 true；此时， mrcoordinator.go 将退出。
  - 退出前，预期收到所有worker的任务请求，统一返回一个任务类型，让其自己关闭
- 任务管理
  - 任务编号
    - 插入任务时生成编号
  - 使用并发队列来管理任务
    - 需要分配任务时，取队头的任务，出队，把任务放到一个临时区域中，设置超时 
      - 如果超时，将任务插入队尾
      - 如果没有超时，自然死亡，并在回执返回时进行下一步处理
  - 收到回执
    - 将回执中的任务插入队尾
  - 统一提交机制
    - 任务完成返回的文件名是临时文件名，由master统一为临时文件重命名   
    - 任务返回需要携带自己的worker编号
  - 超时机制
    - 在任务发放处理时，启动一个计时器
    - 当收到对应任务回执，则向对应channel 发送一个消息
      - 将计时器与worker index挂钩
  - 任务状态转换机制
    - 从Map到Reduce的转换
      - 条件
        - Map任务全部完成
          - Map任务队列中已经没有元素了（通过读取队列长度判断）
          - 没有正在等待判断是否超时的Map任务（难点）
            - 朴素方法：直接等待十秒，然后再去看看Map任务队列
            - 朴素方法，查看中间文件
              - os.ReadDir读取目录下文件名，然后使用正则表达式来看是否全部任务已经生成了最终提交文件
            - 直接查看是否所有worker状态都是idle或者down
    - 从Reduce到Complete的转换
  - 边界情况处理
    - 当M任务已经全部派出，但状态没有转换时，会有新的任务请求发进来
      - 而现在的设计会导致无法切换到reduce，因为他会派发空任务，然后将空任务因为超时重新入队
        - 尝试的办法：当任务列表为空，对任务请求的回应是一个类型为wait的任务包，这个任务包不会设置超时
          - 当worker收到wait任务包，会睡眠一秒
# Worker
Worker数据结构
- 自己的id，用于在最初向Master注册自身
- 正在执行的任务（用于发送回应包）
- map和reduce的函数指针
- 自身状态


Worker行为
- 当自身空闲时，请求分配任务
  - map任务行为（主要是文件处理）
    - 在请求任务后，会收到一个包含文件名的数组
      - 根据任务类型，使用文件名调用函数
      - 如果是map，将输出的kv对数据转换为json，并进行临时命名
      - 如果是reduce，读取对应的数据输入
- 当作业完全完成时，工作进程应该退出。实现此目的的一个简单方法是使用 call() 的返回值：如果工作程序无法联系协调器，则可以假设协调器已退出，因为作业已完成，因此工作器也可以终止。根据您的设计，您可能还会发现协调器可以向工作人员提供“请退出”伪任务很有帮助。
- Map
    - The map phase should divide the intermediate keys into buckets for nReduce reduce tasks, where nReduce is the number of reduce tasks -- the argument that main/mrcoordinator.go passes to MakeCoordinator(). Each mapper should create nReduce intermediate files for consumption by the reduce tasks

# RPC
注册函数
- Worker向Master发送一个注册消息，不包含任何信息，Master收到消息后将Worker加入WorkerList并生成编号，返回编号

Worker状态检测（暂时不需要，如果有一个worker下线了，通过任务超时机制就可以得知，存在冗余）
- worker多开一个routine，定期向master发送心跳包，告知master自己还活着
- 心跳包中有自己的编号

任务发放
- 发送含有任务类型与文件名的消息，返回完成消息，携带完成的worker编号

任务请求
- 发送携带worker编号的消息

# 修修补补
- 测试期待的输出似乎是，mr-sequential的结果被分割开来
  - 使用提供的ihash函数，在转换状态前重新规划reduce任务文件队列
    - map的修改
      - 产生的中间文件MapInter-X-Y，X是输入来源编号，Y是对key进行ihash%nRedece后的编号
      - 因此map的过程应该改为：先创建好8个编号的文件，然后再遍历key的时候对其逐个写入
    - 针对reduce的修改
      - 在master进入reduce任务周期前，将临时文件根据末尾编号重新组成任务组
        - 在初始化时，就根据nReduce创建同等数量的任务组
- 程序在边界情况的正确性还是有问题