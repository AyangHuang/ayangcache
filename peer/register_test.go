package peer

import (
	"fmt"
	"testing"
	"time"
)

const (
	EtcdEndPoint = "127.0.0.1:2379"
)

func TestNewEtcdRegistrationCenterClient(t *testing.T) {
	etcd := NewEtcdRegistrationCenterClient("127.0.0.1:10000", EtcdEndPoint)
	go func() {
		for {
			select {
			case nodes, ok := <-etcd.Notify():
				if ok {
					fmt.Println(nodes)
				} else {
					fmt.Println("close")
					return
				}
			}
		}

	}()
	time.Sleep(20 * time.Second)
	etcd.Close()
	<-make(chan struct{})
}
