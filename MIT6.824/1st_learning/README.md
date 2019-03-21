# MIT6.824 第一次学习记录

课程链接：[https://pdos.csail.mit.edu/6.824/schedule.html](https://pdos.csail.mit.edu/6.824/schedule.html)

这次功利性比较强，所以很多延伸的工作很难展开了，只能跟着课程走一遍罢了。   
之后一定再补回来。

## MapReduce 阅读笔记

[MapReduce 阅读笔记](https://github.com/rsy56640/paper-reading/tree/master/%E5%88%86%E5%B8%83%E5%BC%8F/MapReduce)

## Lab 1

不难，但是对windows很不友好，，完全不能测试，还得搬到腾讯服务器上面，累成狗。。。

有几个坑，记录一下

- 命令行参数 "*" 在windows上没用
- Go还不熟练，容器的cap给多少，各种调用开销怎么算都没有大致的把握
- Go的基础工具库感觉比C++要好很多
- 做到RPC哪里发现 net.Listen全是 unix，心一横打算全网络调用全改成windows，最后越挖越深，，心态崩了，搬到腾讯云centos上测试了
- **论文中说doReduce的时候要排序**，但其实既然reduce是commutative，我就直接从每个文件读出来，然后分到对应的key那里
- 其实**真正的 schedule 完全没有讲**，因为所有file都分散在不同的worker上，需要通过对这些file的分布进行一些计算，把task分到离file尽可能近的worker上
- centos真tm好用，ftp传过去就完事了

