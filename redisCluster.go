package main

import (
	"fmt"
	"strings"
	"time"

	"./lib"
	"github.com/mediocregopher/radix.v2/cluster"
	"github.com/mediocregopher/radix.v2/redis"
)

const (
	ADMIN      = "127.0.0.1:7001"
	TOTALSLOTS = 16384
)

var (
	Infof   = lib.Infof
	Infoln  = lib.Infoln
	Debugf  = lib.Debugf
	Debugln = lib.Debugln
	Errorf  = lib.Errorf
	Errorln = lib.Errorln
)

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

func getCluster() *cluster.Cluster {
	cluster, err := cluster.New(ADMIN)
	if err != nil {
		panic(err)
	}
	return cluster
}
func loadClusterInfo() (masters, slaves map[string]*Node) {
	// load redis-cluster masters&slaves ip/port info
	masters = map[string]*Node{
		"10.0.1.42:7001": &Node{"10.0.1.42", "7001", "", ""},
		"10.0.1.42:7002": &Node{"10.0.1.42", "7002", "", ""},
		"10.0.1.42:7003": &Node{"10.0.1.42", "7003", "", ""},
	}
	slaves = map[string]*Node{
		"10.0.1.42:7004": &Node{"10.0.1.42", "7004", "", "10.0.1.42:7001"},
		"10.0.1.42:7005": &Node{"10.0.1.42", "7005", "", "10.0.1.42:7002"},
		"10.0.1.42:7006": &Node{"10.0.1.42", "7006", "", "10.0.1.42:7003"},
	}
	return
}

func joinCluster(nodes, masters, slaves map[string]*Node) {
	Infoln("===========================  Join Cluster ===========================")
	// set up admin
	c, err := redis.Dial("tcp", ADMIN)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	// meet all masters and slaves
	for k, v := range masters {
		c.Cmd("CLUSTER", "MEET", v.IP, v.Port)
		nodes[k] = v
	}

	for k, v := range slaves {
		c.Cmd("CLUSTER", "MEET", v.IP, v.Port)
		nodes[k] = v
	}

	// construct nodeID-address map
	time.Sleep(time.Duration(1500 * time.Millisecond))
	resp := c.Cmd("CLUSTER", "NODES")
	if err != nil {
		panic(err)
	}
	str, err := resp.Str()
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
		Infoln(v)
	}
}

// set up master-slave relationship
func setupMasterSlave(masters, slaves map[string]*Node) {
	Infoln("=========================== Msater-Slave ===========================")
	var resp *redis.Resp
	for k, v := range slaves {
		conn, err := redis.Dial("tcp", k)
		if err != nil {
			panic(err)
		}
		resp = conn.Cmd("CLUSTER", "REPLICATE", masters[v.Master].NodeId)
		Debugln("CLUSTER REPLICATE RESULT", resp)
		conn.Close()
	}
}

func allocHashSlots(masters map[string]*Node) {
	Infoln("=========================== Alloc Slots ===========================")
	var resp *redis.Resp
	each := TOTALSLOTS / len(masters)
	Debugf("Total Slots: %v, number of masters: %v, number of each hash slots: %v", TOTALSLOTS, len(masters), each)
	var array [TOTALSLOTS]int
	for n, _ := range array {
		array[n] = n
	}
	slice := make([][]int, len(masters))
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
		resp = conn.Cmd("CLUSTER", "ADDSLOTS", slice[n])
		Debugln("CLUSTER ADDSLOTS RESULT", resp)
		conn.Close()
		n++
	}
}

func verifyRedis() {
	Infoln("=========================== Redis Verification  ===========================")
	var resp *redis.Resp
	cluster := getCluster()
	defer cluster.Close()
	resp = cluster.Cmd("CLUSTER", "NODES")
	str, err := resp.Str()
	if err != nil {
		panic(err)
	}
	arr := strings.Split(str, "\n")
	for _, v := range arr {
		if v != "" {
			Infoln(v)
		}
	}
	Infoln("=========================== Simple Set-Get  ===========================")
	Debugln("SET foo bar")
	resp = cluster.Cmd("SET", "foo", "bar")
	Debugln("GET foo")
	resp = cluster.Cmd("GET", "foo")
	Debugln("===>", resp)
}

func main() {
	lib.SetLevel(lib.DebugLevel)
	nodes := make(map[string]*Node)
	masters, slaves := loadClusterInfo()
	joinCluster(nodes, masters, slaves)
	setupMasterSlave(masters, slaves)
	allocHashSlots(masters)
	verifyRedis()
}
