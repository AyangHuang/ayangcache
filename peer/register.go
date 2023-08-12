package peer

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/v3"
	etcdMutex "go.etcd.io/etcd/client/v3/concurrency"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LockPrefix = "/ayangcache/lock"
	NodeSeqKey = "/ayangcache/seq"
	NodePre    = "/ayangcache/node"
	// NodeTTL 10s 无续约则过期
	NodeTTL = 10
)

type addr = string

type Node struct {
	Addr addr
	// 保证一致性 hash 初始化顺序所有节点一致
	NodeSeq int
}

type RegistrationCenterClient interface {
	// Notify 有新增、删除都会返回当前全部的节点，且必须按照 NodeSsq 从小到大排序
	Notify() <-chan []Node
	Close()
}

type etcdRegistrationCenterClient struct {
	etcdClient  *clientv3.Client
	local       Node
	activeNodes []Node
	// 保证写入 notify 和 close notify 的并发安全，因为写入 closed chan 会 panic，即使有 select 也不能阻止写入 closed chan
	notifyMutex sync.Mutex
	notify      chan []Node
	// 停止续期（由于 etcd 提供的 api 是用 context 来控制，所以。。。）
	cancel context.CancelFunc
	// 全局关闭控制
	closed chan struct{}
}

// NewEtcdRegistrationCenterClient 写完突然发现，有点面向过程的写法哈哈
func NewEtcdRegistrationCenterClient(localAddr, etcdEndPoint addr) *etcdRegistrationCenterClient {

	// 初始化 etcd 和分布式锁
	etcdClient, mutex := initEtcd(etcdEndPoint)
	// 利用分布式锁获取节点序号
	nodeSeq := getNodeSeq(etcdClient, mutex)

	rcc := &etcdRegistrationCenterClient{
		etcdClient:  etcdClient,
		local:       Node{Addr: localAddr, NodeSeq: nodeSeq},
		activeNodes: make([]Node, 0),
		notify:      make(chan []Node, 64),
		closed:      make(chan struct{}),
	}

	// 注册服务，并启动心跳
	cancel := rcc.register(etcdClient, nodeSeq, localAddr)
	rcc.cancel = cancel

	// 第一次发现服务和长期监听服务
	rcc.watch(etcdClient)

	return rcc
}

func (rcc *etcdRegistrationCenterClient) Notify() <-chan []Node {
	return rcc.notify
}

func (rcc *etcdRegistrationCenterClient) Close() {
	close(rcc.closed)
	// 停止续约
	rcc.cancel()
	// 注册中心删除服务
	rcc.unRegister()
	// close notifyChan
	rcc.notifyClose()
}

func (rcc *etcdRegistrationCenterClient) register(etcdClient *clientv3.Client, NodeSeq int, addr addr) context.CancelFunc {
	var err error

	// 创建租约
	leaseResp, err := etcdClient.Grant(context.TODO(), NodeTTL)
	if err != nil {
		panic(err.Error())
	}

	// 给租约无限续期，即心跳
	ctx, cancel := context.WithCancel(context.TODO())
	respChan, err := etcdClient.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		panic(err.Error())
	}
	go func() {
		for {
			select {
			// 消费心跳的 resp，这里假定心跳永远成功（实际环境心跳失败可能立刻重新重试？？？），不做任何处理
			case _ = <-respChan:
			case <-rcc.closed:
				return
			}
		}
	}()

	key := formatKey(NodeSeq)
	// 带上租约，注册到 etcd 中
	_, err = etcdClient.Put(context.TODO(), key, addr, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		panic(err.Error())
	}

	return cancel
}

func (rcc *etcdRegistrationCenterClient) unRegister() {
	rcc.etcdClient.Delete(context.Background(), formatKey(rcc.local.NodeSeq))
}

func (rcc *etcdRegistrationCenterClient) watch(etcdClient *clientv3.Client) {
	// 监听
	watchChan := etcdClient.Watch(context.TODO(), NodePre, clientv3.WithPrefix())

	// 获取全部节点，初始化 activeNodes
	resp, err := etcdClient.Get(context.TODO(), NodePre, clientv3.WithPrefix())
	if err != nil {
		panic(err.Error())
	}
	for _, kv := range resp.Kvs {
		rcc.putNode(Node{
			Addr:    string(kv.Value),
			NodeSeq: formatGetNodeSeq(string(kv.Key)),
		})
	}
	// 第一次通知更新
	rcc.notify <- rcc.activeNodes

	go func() {
		for {
			select {
			case resp := <-watchChan:
				for _, event := range resp.Events {
					switch event.Type {
					// 新加入节点
					case mvccpb.PUT:
						rcc.putNode(Node{
							Addr:    string(event.Kv.Value),
							NodeSeq: formatGetNodeSeq(string(event.Kv.Key)),
						})
						// 删除节点
					case mvccpb.DELETE:
						rcc.delNode(formatGetNodeSeq(string(event.Kv.Key)))
					}
				}
			case <-rcc.closed:
				return
			}

			// 通知更新
			rcc.notifySend()
		}

	}()
}

func initEtcd(endpoint addr) (*clientv3.Client, sync.Locker) {
	etcd, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		panic(err.Error())
	}

	session, err := etcdMutex.NewSession(etcd)
	if err != nil {
		panic(err.Error())
	}

	etcdLock := etcdMutex.NewLocker(session, LockPrefix)

	return etcd, etcdLock
}

func getNodeSeq(etcdClient *clientv3.Client, etcdMutex sync.Locker) int {
	var err error

	// 利用 etcd 锁，保护 NodeSeq 并发安全
	etcdMutex.Lock()
	// 注意：这里只是简单的加锁上锁（测试得出： etcd Lock 内部如果长时间不释放，会自动释放）
	// 实际环境需要看门狗机制等保证锁的正确释放
	defer etcdMutex.Unlock()
	resp1, err := etcdClient.Get(context.TODO(), NodeSeqKey)
	if err != nil {
		panic(err.Error())
	}

	// 第一次
	if resp1.Count == 0 {
		_, err = etcdClient.Put(context.TODO(), NodeSeqKey, "1")
		if err != nil {
			panic(err.Error())
		}
		return 1
	}

	nodeSeq, _ := strconv.Atoi(string(resp1.Kvs[0].Value))
	nodeSeq++
	_, err = etcdClient.Put(context.TODO(), NodeSeqKey, strconv.Itoa(nodeSeq))
	if err != nil {
		panic(err.Error())
	}

	return nodeSeq
}

func (rcc *etcdRegistrationCenterClient) notifyClose() {
	rcc.notifyMutex.Lock()
	close(rcc.notify)
	rcc.notifyMutex.Unlock()
}

func (rcc *etcdRegistrationCenterClient) notifySend() {
	rcc.notifyMutex.Lock()
	defer rcc.notifyMutex.Unlock()
	// double check
	select {
	case <-rcc.closed:
		return
	default:
	}

	rcc.notify <- rcc.activeNodes
}

func (rcc *etcdRegistrationCenterClient) putNode(newNode Node) {
	// 有序，所以通过二分的方式加快查找
	index := sort.Search(len(rcc.activeNodes), func(i int) bool {
		return rcc.activeNodes[i].NodeSeq >= newNode.NodeSeq
	})

	// 如果超出（即没有该节点）或者 > 该节点（同样是没有该节点）
	if index >= len(rcc.activeNodes) || rcc.activeNodes[index].NodeSeq != newNode.NodeSeq {
		rcc.activeNodes = append(rcc.activeNodes, Node{})
		copy(rcc.activeNodes[index+1:], rcc.activeNodes[index:])
		rcc.activeNodes[index] = newNode
	} else { // 找到，替换即可
		rcc.activeNodes[index] = newNode
	}
}

func (rcc *etcdRegistrationCenterClient) delNode(targetNodeSeq int) {
	index := sort.Search(len(rcc.activeNodes), func(i int) bool {
		return rcc.activeNodes[i].NodeSeq >= targetNodeSeq
	})

	if index < len(rcc.activeNodes) && rcc.activeNodes[index].NodeSeq == targetNodeSeq {
		copy(rcc.activeNodes[index:], rcc.activeNodes[index+1:])
		rcc.activeNodes = rcc.activeNodes[:len(rcc.activeNodes)-1]
	}
}

func formatKey(nodeSeq int) string {
	return fmt.Sprintf(NodePre+"/%d", nodeSeq)
}

func formatGetNodeSeq(key string) int {
	nodeSeqStr := strings.Split(key, "/")[3]
	nodeSeq, _ := strconv.Atoi(nodeSeqStr)
	return nodeSeq
}
