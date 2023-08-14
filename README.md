# ayangcache

此项目是学习 [极客兔兔的 geecache](https://github.com/geektutu/7days-golang)，geecache 又是模仿 memcached 的作者写的一个 Go 项目 [groupcache](https://github.com/golang/groupcache)  

然后我的项目在 geecache 的基础上，进行了魔改，魔改如下：

1. 项目模块划分更加细致：缓存模块、传输模块和分布式模块  
2. 传输模块
   1. 摒弃直接使用 net.http 进行通信，自己实现了客户端和服务端
   2. 可选择使用 Json 或 Protobuf 进行序列化
   3. 传输层使用 TCP 长连接，采用 goroutine 实现读写分离，全双工、异步响应（区别于 HTTP1.1 的半双工）
3. 缓存模块：缓存模块是学习了 [ristretto](https://github.com/dgraph-io/ristretto)，对着写（bushi，抄）了一遍    
   该缓存库主要采用数据分片+批量+异步相结合的方法，大大提高并发度
4. 分布式模块：  
   引入了 ETCD 作为服务注册、发现中心，真正实现分布式，支持动态扩容和缩容

感觉这个项目是一个很好的练手项目，有服务端客户端通信的实践、消息序列化的实践、缓存算法的实践、缓存如何提高并发度的实践、一致性 hash 算法的实践、服务注册/发现的实践、特别是 goroutine 的丰富运用。总之，是一个比较不错的学习项目  

ps：如果没有安装 ETCD，可以直接用 main branch，没有服务注册/发现功能，不支持动态扩容和缩容
