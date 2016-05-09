package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	rd "github.com/Apsalar/redis-go-cluster"
	"github.com/garyburd/redigo/redis"
)

const (
	ADMIN      = "127.0.0.1:7000"
	TOTALSLOTS = 16384
)

var (
	Info  *log.Logger
	Debug *log.Logger
	Warn  *log.Logger
	Error *log.Logger
)

func Init(infoHandle, debugHandle, warnHandle, errorHandle io.Writer) {
	Info = log.New(infoHandle, "[INFO]  ", log.Ldate|log.Ltime|log.Lshortfile)
	Debug = log.New(debugHandle, "[DEBU] ", log.Ldate|log.Ltime|log.Lshortfile)
	Warn = log.New(warnHandle, "[WARN]  ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(errorHandle, "[ERRO] ", log.Ldate|log.Ltime|log.Lshortfile)
}

func init() {
	Init(os.Stdout, os.Stdout, os.Stdout, os.Stderr)
}

type Node struct {
	IP     string
	Port   string
	NodeId string
	Master string
}

func (n *Node) String() string {
	if n.Master == "" {
		return fmt.Sprintf("%s@%s:%s", n.NodeId, n.IP, n.Port)
	} else {
		return fmt.Sprintf("%s@%s:%s ---> %s", n.NodeId, n.IP, n.Port, n.Master)

	}
}

func getCluster(connTimeout, readTimeout, writeTimeout time.Duration) *rd.Cluster {
	cluster, err := rd.NewCluster(
		&rd.Options{
			StartNodes: []string{
				"127.0.0.1:7000",
				"127.0.0.1:7001",
				"127.0.0.1:7002",
				"127.0.0.1:7003",
				"127.0.0.1:7004",
				"127.0.0.1:7005",
			},
			ConnTimeout:  connTimeout,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			/*
				ConnTimeout:  100 * time.Microsecond,
				ReadTimeout:  3 * time.Microsecond,
				WriteTimeout: 3 * time.Microsecond,
			*/
			KeepAlive: 16,
			AliveTime: 60 * time.Second,
		})
	if err != nil {
		panic(err)
	}
	return cluster
}
func loadClusterInfo() (masters, slaves map[string]*Node) {
	// load redis-cluster masters&slaves ip/port info
	masters = map[string]*Node{
		"127.0.0.1:7000": &Node{"127.0.0.1", "7000", "", ""},
		"127.0.0.1:7001": &Node{"127.0.0.1", "7001", "", ""},
		"127.0.0.1:7002": &Node{"127.0.0.1", "7002", "", ""},
	}
	slaves = map[string]*Node{
		"127.0.0.1:7003": &Node{"127.0.0.1", "7003", "", "127.0.0.1:7000"},
		"127.0.0.1:7004": &Node{"127.0.0.1", "7004", "", "127.0.0.1:7001"},
		"127.0.0.1:7005": &Node{"127.0.0.1", "7005", "", "127.0.0.1:7002"},
	}
	return
}

func joinCluster(nodes, masters, slaves map[string]*Node) {
	Info.Println("===========================  Join Cluster ===========================")
	// set up admin
	c, err := redis.Dial("tcp", ADMIN)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	// meet all masters and slaves
	for k, v := range masters {
		c.Do("CLUSTER", "MEET", v.IP, v.Port)
		nodes[k] = v
	}

	for k, v := range slaves {
		c.Do("CLUSTER", "MEET", v.IP, v.Port)
		nodes[k] = v
	}

	// construct nodeID-address map
	time.Sleep(time.Duration(1000 * time.Millisecond))
	str, err := redis.String(c.Do("CLUSTER", "NODES"))
	if err != nil {
		panic(err)
	}
	arr := strings.Split(str, "\n")
	for _, v := range arr {
		fields := strings.Split(v, " ")
		if fields != nil && len(fields) == 8 {
			if nodes[fields[1]].String() != "" {
				nodes[fields[1]].NodeId = fields[0]
			}
		}
	}
	for _, v := range nodes {
		Info.Println(v)
	}
}

// set up master-slave relationship
func setupMasterSlave(masters, slaves map[string]*Node) {
	Info.Println("=========================== Msater-Slave ===========================")
	for k, v := range slaves {
		conn, err := redis.Dial("tcp", k)
		if err != nil {
			panic(err)
		}
		resp, err := conn.Do("CLUSTER", "REPLICATE", masters[v.Master].NodeId)
		Debug.Println("CLUSTER REPLICATE RESULT", resp)
		conn.Close()
	}
}

func allocHashSlots(masters map[string]*Node) {
	Info.Println("=========================== Alloc Slots ===========================")
	each := TOTALSLOTS / len(masters)
	Debug.Printf("Total Slots: %v, number of masters: %v, number of each hash slots: %v", TOTALSLOTS, len(masters), each)
	var array [TOTALSLOTS]string
	for n, _ := range array {
		array[n] = strconv.Itoa(n)
	}
	slice := make([][]string, len(masters))
	var n int = 0
	for n = 0; n < (len(masters) - 1); n++ {
		slice[n] = array[n*each : (n+1)*each]
	}
	slice[n] = array[n*each:]
	n = 0
	for k, _ := range masters {
		conn, err := redis.Dial("tcp", k)
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		for _, v := range slice[n] {
			err := conn.Send("CLUSTER", "ADDSLOTS", v)
			if err != nil {
				panic(err)
			}
		}
		conn.Flush()
		resp, err := conn.Receive()
		Debug.Println("CLUSTER ADDSLOTS RESULT", resp)
		n++
	}
	time.Sleep(time.Duration(500 * time.Millisecond))
}

func verifyCluster() {
	Info.Println("=========================== Redis Verification  ===========================")
	cluster := getCluster(100*time.Microsecond, 100*time.Microsecond, 100*time.Microsecond)
	str, err := rd.String(cluster.Do("CLUSTER", "NODES"))
	if err != nil {
		panic(err)
	}
	arr := strings.Split(str, "\n")
	for _, v := range arr {
		if v != "" {
			Info.Println(v)
		}
	}

	Info.Println("=========================== Simple Set-Get  ===========================")
	Debug.Println("SET foo bar")
	resp, err := cluster.Do("SET", "foo", "bar")
	if err == nil {
		Debug.Println(resp, ", then GET foo")
		str, err = rd.String(cluster.Do("GET", "foo"))
		Debug.Println("===>", str)
	}
	cluster.Close()
}

func testTimeout() {
	Info.Println("=========================== Timeout Testing  ===========================")
	cluster := getCluster(100*time.Microsecond, 2*time.Microsecond, 2*time.Microsecond)
	Debug.Println("GET foo")
	str, err := rd.String(cluster.Do("GET", "foo"))
	Debug.Println("===>", str, "with", err)
	cluster.Close()
}

func setupCluster() {
	nodes := make(map[string]*Node)
	masters, slaves := loadClusterInfo()
	joinCluster(nodes, masters, slaves)
	setupMasterSlave(masters, slaves)
	allocHashSlots(masters)
}
func main() {
	timeout := flag.Bool("t", false, "timeout testing")
	flag.Parse()
	if *timeout {
		testTimeout()
		return
	}
	setupCluster()
	verifyCluster()
}
