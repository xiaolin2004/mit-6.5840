# 要求
实现线性化的内存上kv存储
- 线性化定义：
   此处沿用课程表示方式
   1. the x-axis is real time
    |- indicates the time at which client sent request
    -| indicates the time at which the client received the reply
  "Wx1" means "write value 1 to record x" -- put(x, 1)
  "Rx1" means "a read of record x yielded value 1" -- get(x) -> 1
  可以形成如下的表示图
  example:
    C1: |-Wx1-| |-Wx2-|
    C2:   |---Rx2---|
  2. 在客户端的视角，请求-收到回复是一个带有时间长度的事件
  3. 而在服务端的视角，需要将每一个服务端视角的操作事件归约为一个原子操作，然后按照合理的顺序排列并处理这些操作
  4. 对于多台客户端，他们所看到的对服务端的操作顺序必须是一致的，不能出现C1看到Wx1，Wx2而C2看到Wx2，Wx1
# 实验架构
- Clerk作为中间层，处理Client和Server之间的交互
# 设计
- 在server端使用一个带锁任务队列，clerk发起rpc请求，把收到的任务包入队
- 任务包数据结构
  - args
  - reply
  - 任务类型
  - 任务唯一标识（用于解决丢弃重传）
- server由两个模块组成：调度器（目前核心数据结构是一个带锁队列）以及K-V存储（直接使用map数据结构）
# 问题：
  - 应该在哪一端实现线性化的逻辑
    - Your server must arrange that application calls to Clerk Get/Put/Append methods be linearizable.应该实现在服务器端
  - 如何实现线性化的逻辑
  - 文件夹中的config和test中的代码在干嘛 
    只是测试中用来获取中间过程信息的代码
  - 如何实现丢弃重传包的辨认
    - 唯一标识请求包
      - Clerk在没有收到回复之后，会重新传包，可以在此时给请求包打上时间戳或者递增的唯一标识  
      - 但是如何确定这个是之前包的重传包，且如何令服务器知道这个包是否被执行过
        - 可以在任务中打上重传标记，重传+递增标识来识别执行
        - 使用一个数据结构储存已经处理过的任务编号，实现过期机制
        - GPT4:在这种情况下，一种常见的解决方案是使用"幂等操作"的概念。幂等操作是指无论进行多少次操作，结果都是相同的。也就是说，如果一个客户端重复发出相同的请求，系统会识别这个请求已经执行过，不再进行处理，而是直返回已经执行过的结果。具体到您的问题，每个请求可以分配一个唯一的ID（例如，UUID或者时间戳+客户端ID等），这个ID作为请求的一部分发送到服务器。服务器在执行请求之前先检查这个ID是否已经存在于一个已经被处理的请求的列表中。如果ID已经存在，那么表明这个请求已经被处理过，服务器只需返回之前处理过的结果；否则，服务器会执行这个请求，并将这个ID添加到已处理的请求列表中。然后需要做的就是定期清理这个已处理的请求ID列表，因为随着时间和请求的积累，这个列表可能会变得很大。具体的清理策略，你可以设定一个过期时间，比如说超过这个时间的请求ID，就从列表中删除。另外，这种方案最重要的一点就是要确保这个检查-执行-添加ID的过程是原子的，这是为了防止并发请求的情况下出现问题。你可以使用锁或其他并发控制机制来实现。


# 对象行为
## client
## server
- 任务有序化
  - 通过对锁的争抢来实现，抢锁就是序列化过程，通过队列数据结构见序列固定下来
- 操作执行记录
  - 将每一个修改操作写入一个数据结构中，并设置清理时间
  - 每当收到一个修改操作，搜索是否已经被执行
- Put
  - 如果该值存在，则替换，如果不存在，则直接新增
- Append
  - 如果value存在，则将请求中的value append到原value的末尾，如果不存在，就新建一个并直接存入请求中的value

# 性能瓶颈
- 大量append请求导致缓存中的value平方级增长
  - 增量存储
    - 存入对应区间首尾index
      - 如果被put修改呢？ 
    - 将增量存入一个数组中，并标识是第几次append，然后通过这个前向组合出old
      - 当遇到put，清空这个数组，并重新开始记录
    - 另一种思路是，只储存修改请求的操作序列，缓存查询的时候通过计算得到当时的数据

# 进度缓存
性能瓶颈没有解决，测试结果如下：
Test: one client ...
  ... Passed -- t  4.7 nrpc 40247 ops 40247
Test: many clients ...
info: linearizability check timed out, assuming history is ok
  ... Passed -- t  5.8 nrpc 113169 ops 113169
Test: unreliable net, many clients ...
  ... Passed -- t  3.3 nrpc  1164 ops  944
Test: concurrent append to same key, unreliable ...
  ... Passed -- t  0.3 nrpc    68 ops   52
Test: memory use get ...
  ... Passed -- t  0.3 nrpc     5 ops    0
Test: memory use put ...
  ... Passed -- t  0.2 nrpc     2 ops    0
Test: memory use append ...
  ... Passed -- t  0.3 nrpc     2 ops    0
Test: memory use many put clients ...
  ... Passed -- t  8.1 nrpc 100000 ops    0
Test: memory use many get client ...
  ... Passed -- t  9.1 nrpc 100001 ops    0
Test: memory use many appends ...
--- FAIL: TestMemManyAppends (1.19s)
    test_test.go:599: error: server using too much memory m0 380456 m1 96754608
FAIL
exit status 1
FAIL    6.5840/kvsrv    35.411s