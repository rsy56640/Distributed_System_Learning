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

## 