# MIT6.824 第一次学习记录

课程链接：[https://pdos.csail.mit.edu/6.824/schedule.html](https://pdos.csail.mit.edu/6.824/schedule.html)

这次功利性比较强，所以很多延伸的工作很难展开了，只能跟着课程走一遍罢了。   
之后一定再补回来。

## MapReduce 阅读笔记

[MapReduce 阅读笔记](https://github.com/rsy56640/paper-reading/tree/master/%E5%88%86%E5%B8%83%E5%BC%8F/MapReduce)

## Lab 1 MapReduce

不难，但是对windows很不友好，，完全不能测试，还得搬到腾讯服务器上面，累成狗。。。

有几个坑，记录一下

- 命令行参数 "*" 在windows上没用
- Go还不熟练，容器的cap给多少，各种调用开销怎么算都没有大致的把握
- Go的基础工具库感觉比C++要好很多
- 做到RPC哪里发现 net.Listen全是 unix，心一横打算全网络调用全改成windows，最后越挖越深，，心态崩了，搬到腾讯云centos上测试了
- **论文中说doReduce的时候要排序**，但其实既然reduce是commutative，我就直接从每个文件读出来，然后分到对应的key那里
- 其实**真正的 schedule 完全没有讲**，因为所有file都分散在不同的worker上，需要通过对这些file的分布进行一些计算，把task分到离file尽可能近的worker上
- centos真tm好用，ftp传过去就完事了

## Lec 2 RPC

`goroutnie`s are pretty cheap, use `channel` and `WainGroup` to coordinate.   
爬虫：1. IO并发，增加爬取速度；2. 每一个URL爬取一次

goroutine 之间的通信机制值得研究一番（所谓 "*Do not communicate by sharing memory; instead, share memory by communicating.*"）   
目前了解到的通信手段：

- `sync.WaitGroup`
- 任务计数
- 返回值计数
- `channel` 通知另一方停止，己方完成后 send to `channel`，另一方 `select`

使用 共享数据（`mutex`） 和 `channel` 的时机：

- 状态：共享数据（`mutex`）
- 通信：`channel`
- 等待事件：`channel`

### RPC 调用

client

```go   
    client, err := rpc.Dial("tcp", "host_addr")
    client.Call("method_name", &args, &reply)
```

server

```go   
    rpcs := rpc.NewServer()
    rpcs.Register(kv)
    l, e := net.Listen("tcp", "host_addr")
    go func() {
        for {
            conn, err := l.Accept()
            if err == nil {
                go rpcs.ServeConn(conn)
            } else {
                break
            }
        }
        l.Close()
    }()
```

server 的方法必须上锁，因为 RPC lib 为每个 request 创建一个 goroutine

RPC 问题：dup request（如何保证幂等性）   
Solution：时间戳，等

## GFS 阅读笔记

[GFS 阅读笔记](https://github.com/rsy56640/paper-reading/tree/master/%E5%88%86%E5%B8%83%E5%BC%8F/GFS)

## Lec 3 GFS
- [mit6.824 lec3 GFS](https://pdos.csail.mit.edu/6.824/notes/l-gfs-short.txt)
- [mit6.824 GFS-FAQ](https://pdos.csail.mit.edu/6.824/papers/gfs-faq.txt)

即使 append offset 不一致也无所谓，app 几乎总是读整个文件（换句话说，很少指定读操作的文件偏移量）

关于 replica 的距离，google 将距离近的机器分配相近的IP地址，通过IP地址判断 replica 距离

关于 consistency 和 performance 的 trade-off

## The design of a practical system for fault-tolerant virtual machines 阅读笔记

[The design of a practical system for fault-tolerant virtual machines 阅读笔记](https://github.com/rsy56640/paper-reading/tree/master/%E5%88%86%E5%B8%83%E5%BC%8F/The%20design%20of%20a%20practical%20system%20for%20fault-tolerant%20virtual%20machines)

## Lec 4 Primary/Backup Replication

- [mit6.824 lec4 Primary/Backup Replication](https://pdos.csail.mit.edu/6.824/notes/l-vm-ft.txt)
- [mit6.824 VMware FT FAQ](https://pdos.csail.mit.edu/6.824/papers/vm-ft-faq.txt)

GFS 与 VM-FT 比较：

- VM-FT 提供强一致性，并且对 client 透明
- GFS 只提供存储的容错，对一致性没有保证，针对大文件顺序读写做优化

bounce buffer 作用：为了屏蔽硬件DMA带来的不确定性读取操作   
FT 先把要读取的内容 copy 到 bounce buffer，primary 这时无法访问；FT hypervisor 在这里 interrupt 并记录日志（在这里中断），然后 FT 把 bounce buffer 的内容 copy 到 primary's memory。backup 在同样的地方中断，然后读数据。

## Raft 阅读笔记

[Raft阅读笔记](https://github.com/rsy56640/paper-reading/tree/master/%E5%88%86%E5%B8%83%E5%BC%8F/raft)

## Lec 5 Raft

- [mit6.824 lec 5 Raft](https://pdos.csail.mit.edu/6.824/notes/l-raft.txt)
- [mit6.824 Raft FAQ](https://pdos.csail.mit.edu/6.824/papers/raft-faq.txt)

性能上的一些问题：   
1. 没有 batch write   
2. Append 每次1个 entry   
3. snapshot 过大，还有传输问题   

## Lec 6 Raft2

- [mit6.824 lec 6 Raft2](https://pdos.csail.mit.edu/6.824/notes/l-raft2.txt)
- [mit6.824 Raft2 FAQ](https://pdos.csail.mit.edu/6.824/papers/raft2-faq.txt)

## Lab 2A: Raft leader election

> 代码可以在 commit 记录中找到

说实话体验不是很好，我看网上很多人到 lab2就不做了，之前以为是有难度，但其实不然。主要问题在于，它给了 code base skeleton，但是几乎没有什么说明，我一开始根本都不知道我的哪些函数会被调用，一脸懵逼。而且 raft 是一个整体，它给了一个框架但其实很多东西是互相牵制的，又没说明，刚开始心态就崩了。。。举个例子：它的 Raft 结构体里面没有 timer，这个要自己加，但是整体调用逻辑它写死了，我都不知道它怎么调用，调用谁。我还是看了比人的实现才知道这是咋回事，然后又不熟悉 go，总共写了2天才过了 lab2A。   
记录一下踩过的 go 的坑： `timer.Afterfunc(duration, func) *Timer`，这个函数返回一个 timer，到时自动调用 `func`，我一开始以为 resetTimer 直接重新赋值就好，但是**之前的那个其实没有关掉**。就导致一个 follower 不断地开始 canvass，term 瞬间上万。

总结一下我的实现思路：

- 首先进入 `Make()` 初始化为 follower，并且设置 timer
- timer 自动触发 `rf.candidateRequestVote()`，开始拉选票
- 所有的接受消息处理例程都要考虑到 network delay，也就是说收到的消息是过时的，这个要正确处理
  - candidate 处理 vote reply：当这个消息 delay 时，**自己已经可能不是candidate了**
  - server 处理 requestVoteRPC：正常走，没啥说的
  - leader 处理 appendLog reply：**有可能自己不是leader了**，或者**可能是自己之前的appendRPC**，要正确更新 `nextIndex` 和 `matchIndex`
  - server 处理 appendLogRPC：正常走
- 我用了一个自动触发选举的 timer： `rf.electionTimer = time.AfterFunc(rf.electionTimeout, func() { rf.candidateRequestVote() })`，记得每次重设前先 `rf.electionTimer.Stop()`
- election 成功后，就成为 leader，并开启一个服务例程，用来持续的发送 heartbeat，如果从 leader 变为 follower，记得关闭这个 heartbeat 服务

## Lab 2B: AppendEntry RPC

> 代码可以在 commit 记录中找到

还是有些坑的，，我之前没有用到 `applyCh`，导致完全不懂它在怎么测试，后来我人肉跟踪进去，发现他维护了我的 apply 的 log，即 `cfg.logs[]`。因为没有用 `applyCh`，这些全是空的，所以一个测试也过不了。。。   
另一个问题是**什么时候 reset timer**：结论是只有 **RequestVoteRPC中确定了vote 或 自己的term小且自己是leader** 或者 **AppendEntryRPC中对比prevIndex和prevTerm成功** 时才 reset timer。这个bug我搞了挺久才发现，把日志全部打出来，然后一点点跟踪，最后发现 RequestVote 在 reject 的时候那个 server 也 reset timer。然而总共就2个server，一个log旧的 timer 时间短，一直让那个应该发起 canvass 的 server reset timer。。。对着这个“[raft动画](https://raft.github.io/)”玩了好久终于明白问题在哪了。（这个模拟好像不支持网络分区）

总结一下实现思路：

- client 请求直接进入 `Start()`，然后返回就行（没显式调用 append，因为我的 `heartbeatRoutine()` 自动尝试同步 log）
- 我这里面的有3个持续性task
  - timer：自动触发 `candidateRequestVote()`。（除了 leader 都有这个，candidate 选举成功时，stop timer）
  - 所有 server：`applyEntryRoutine()`，用条件变量阻塞，直到 commitIndex > lastApplied，然后 apply 到自己的 state machine 里面（在 lab 里，就是发送到 `applyCh`）
  - leader 有的：`heartbeatRoutine()`，candidate 选举成功时开启，每隔一段时间就发起 `sendAppendEntryRPC`，如果对应的 server 没啥发的，就发空的，代表 heartbeat。还没同步到的 server 就一步一步往前尝试同步
- 然后就是那个bug了：什么时候 reset timer，上面已经讨论过了
- 还要注意的一点就是 network delay，当你收到消息（RPC 或者 RPC-reply）的时候，整个 state 可能已经变了，一些 corner case 要做到正确处理

## Lab 2C: persist and Figure-8

> 代码可以在 commit 记录中找到

我之前写的代码里有2处bug：

- canvass 接收 vote 时有可能 msg delay，也就是说如果一个 server 连续发起两次 canvass，有可能在第二次收到第一次的 vote，所以一定要比较一下 vote 的是哪个 term
- 第二个bug算是对 go 不熟练造成的：AppendLogRPC 需要把多余的 log truncate 掉，因为有可能自己的 log 比 leader 长（自己可能是上一届 leader，有多余的 log）

### 我认为 `TestFigure8Unreliable2C()` 有一个重大问题

先来了一个 `cfg.one()`，然后乱搞（乱序消息，掉线，重连）一波，恢复，再来一个 `cfg.one()`。重点就是后面这个 `cfg.one()`。   
`cfg.one()` 是这个意思：选一个 leader，我们认为它可以就这个 command 达成一致，检测并返回。   
但是现在有个问题：乱搞一波之后 reconnect，紧接着调用 `cfg.one()`。考虑这样一种情况：一个 old-term leader reconnect 之后，被 `cfg.one()` 选中，并承诺完成这个 agreement，然而它马上就会发现自己的 term 是 out-of-date，所以这个 command 就没了。这样 `cfg.one()` 中会认为没有达成一致。   
**我个人的做法是在 reconnect 和 `cfg.one()` 中间等待3s，让 reconnect server 认清一下现状。**
