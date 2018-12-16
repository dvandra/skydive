/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package sflow

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
	//"reflect"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	//"github.com/skydive-project/skydive/filters"
	//"github.com/skydive-project/skydive/ovs"
)

const (
	maxDgramSize = 65535
)

var (
	// ErrAgentAlreadyAllocated error agent already allocated for this uuid
	ErrAgentAlreadyAllocated = errors.New("agent already allocated for this uuid")
)

// Agent describes SFlow agent probe
type Agent struct {
	common.RWMutex
	UUID       string
	Addr       string
	Port       int
	FlowTable  *flow.Table
	Conn       *net.UDPConn
	BPFFilter  string
	HeaderSize uint32
	Node       *graph.Node
	Graph      *graph.Graph
}

// AgentAllocator describes an SFlow agent allocator to manage multiple SFlow agent probe
type AgentAllocator struct {
	common.RWMutex
	portAllocator *common.PortAllocator
	agents        []*Agent
}

// GetTarget returns the current used connection
func (sfa *Agent) GetTarget() string {
	target := []string{sfa.Addr, strconv.FormatInt(int64(sfa.Port), 10)}
	return strings.Join(target, ":")
}

func (sfa *Agent) feedFlowTable() {
	var bpf *flow.BPF

	if b, err := flow.NewBPF(layers.LinkTypeEthernet, sfa.HeaderSize, sfa.BPFFilter); err == nil {
		bpf = b
	} else {
		logging.GetLogger().Error(err.Error())
	}

	var buf [maxDgramSize]byte
	for {
		n, _, err := sfa.Conn.ReadFromUDP(buf[:])
		if err != nil {
			return
		}

		// TODO use gopacket.NoCopy ? instead of gopacket.Default
		p := gopacket.NewPacket(buf[:n], layers.LayerTypeSFlow, gopacket.DecodeOptions{NoCopy: true})
		sflowLayer := p.Layer(layers.LayerTypeSFlow)
		sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
		logging.GetLogger().Infof("Value of p = %s", p)
		//logging.GetLogger().Infof("%d sample captured", sflowPacket.SampleCount)
		if !ok {
			logging.GetLogger().Errorf("Unable to decode sFlow packet: %s", p)
			continue
		}
		if sflowPacket.SampleCount > 0 {
			logging.GetLogger().Infof("Node = %s", sfa.Node)
			//sfa.Graph.AddMetadata(sfa.Node, "Sflow-SampleCount:", sflowPacket.SampleCount)
			logging.GetLogger().Debugf("%d sample captured", sflowPacket.SampleCount)
			logging.GetLogger().Infof("%d sample captured", sflowPacket.SampleCount)
			for _, sample := range sflowPacket.FlowSamples {
				// iterate over a set of Packets as a sample contains multiple records each generating Packets.
				sfa.FlowTable.FeedWithSFlowSample(&sample, bpf)
			}
			var counters []layers.SFlowCounterSample

			//counter := sflowPacket.CounterSamples
			//logging.GetLogger().Infof("counters= %v", counter)
			for _, sample := range sflowPacket.CounterSamples {
				counters = append(counters, sample)
				var records []layers.SFlowRecord
				//var format layers.SFlowCounterRecordType
				//var i uint32
				//var rec [][]StructField
				//var record []StructField

				records = sample.GetRecords()

				for _, record := range records{
					//record := records[i]
					//fmt.Println(record)
					//var record = rec[i]
					//t := record.(struct)
					//var rec t
					//rec := record.(reflect.TypeOf(record))

					//for i,rec := range records{
					//for _,format:= range rec.Format{
					//x := rec
					t := record.(interface{})

					switch t.(type) {
					case layers.SFlowGenericInterfaceCounters:
						var gen layers.SFlowGenericInterfaceCounters
						gen = t.(layers.SFlowGenericInterfaceCounters)
						tr := sfa.Graph.StartMetadataTransaction(sfa.Node)
						defer tr.Commit()
						//setInt64 := func(k ) int64 {
						//	if v, ok := gen.k; ok {
						//		return int64(v.(float64))
						//	}
						//	return 0
						//}
						newInterfaceMetricsFromOVSSFlow := func(gen layers.SFlowGenericInterfaceCounters) *topology.InterfaceMetric {
							return &topology.InterfaceMetric{
								Multicast: int64(gen.IfInMulticastPkts),
								//Collisions:    setInt64(collisions),
								//RxBytes:       setInt64(rx_bytes),
								//RxCrcErrors:   setInt64(IfOutErrors),
								RxDropped: int64(gen.IfInDiscards),
								RxErrors:  int64(gen.IfInErrors),
								//RxFrameErrors: setInt64(rx_frame_err),
								//RxOverErrors:  setInt64(rx_over_err),
								RxPackets: int64(int64(gen.IfInUcastPkts) + int64(gen.IfInMulticastPkts) + int64(gen.IfInBroadcastPkts)),
								//TxBytes:   setInt64(tx_bytes),
								TxDropped: int64(gen.IfOutDiscards),
								TxErrors:  int64(gen.IfOutErrors),
								TxPackets: int64(int64(gen.IfOutUcastPkts) + int64(gen.IfOutMulticastPkts) + int64(gen.IfOutBroadcastPkts)),
							}

						}
						currMetric := newInterfaceMetricsFromOVSSFlow(gen)
						logging.GetLogger().Infof("currMetric= %v", currMetric)
						now := time.Now()
						currMetric.Last = int64(common.UnixMillis(now))
						var prevMetric, lastUpdateMetric *topology.InterfaceMetric

						if ovs, err := sfa.Node.GetField("Ovs"); err == nil {
							prevMetric, ok = ovs.(map[string]interface{})["SFlowMetric"].(*topology.InterfaceMetric)
							if ok {
								lastUpdateMetric = currMetric.Sub(prevMetric).(*topology.InterfaceMetric)
							}
						}
						tr.AddMetadata("Ovs.SFlowMetric", currMetric)
						// nothing changed since last update
						if lastUpdateMetric != nil && !lastUpdateMetric.IsZero() {
							lastUpdateMetric.Start = prevMetric.Last
							lastUpdateMetric.Last = int64(common.UnixMillis(now))
							tr.AddMetadata("Ovs.SFlowLastUpdateMetric", lastUpdateMetric)
						}
						/*case 2:
						case 3:
						case 4:
						case 5:
						case 7:
						case 1001:
						case 1004:
						case 1005:
						case 2203:
						case 2207:*/
					}
				}
			}
			logging.GetLogger().Infof("counters= %v", counters)
			sfa.Graph.Lock()
			defer sfa.Graph.Unlock()
			sfa.Graph.AddMetadata(sfa.Node, "Sflow-Counters", counters)

		}
		/*IfIndex            uint32
		IfType             uint32
		IfSpeed            uint64
		IfDirection        uint32
		IfStatus           uint32
		IfInOctets         uint64
		IfInUcastPkts      uint32
		IfInMulticastPkts  uint32
		IfInBroadcastPkts  uint32
		IfInDiscards       uint32
		IfInErrors         uint32
		IfInUnknownProtos  uint32
		IfOutOctets        uint64
		IfOutUcastPkts     uint32
		IfOutMulticastPkts uint32
		IfOutBroadcastPkts uint32
		IfOutDiscards      uint32
		IfOutErrors        uint32
		IfPromiscuousMode  uint32
		Collisions        int64 `json:"Collisions,omitempty"`
		Multicast         int64 `json:"Multicast,omitempty"`
		RxBytes           int64 `json:"RxBytes,omitempty"`
		RxCompressed      int64 `json:"RxCompressed,omitempty"`
		RxCrcErrors       int64 `json:"RxCrcErrors,omitempty"`
		RxDropped         int64 `json:"RxDropped,omitempty"`
		RxErrors          int64 `json:"RxErrors,omitempty"`
		RxFifoErrors      int64 `json:"RxFifoErrors,omitempty"`
		RxFrameErrors     int64 `json:"RxFrameErrors,omitempty"`
		RxLengthErrors    int64 `json:"RxLengthErrors,omitempty"`
		RxMissedErrors    int64 `json:"RxMissedErrors,omitempty"`
		RxOverErrors      int64 `json:"RxOverErrors,omitempty"`
		RxPackets         int64 `json:"RxPackets,omitempty"`
		TxAbortedErrors   int64 `json:"TxAbortedErrors,omitempty"`
		TxBytes           int64 `json:"TxBytes,omitempty"`
		TxCarrierErrors   int64 `json:"TxCarrierErrors,omitempty"`
		TxCompressed      int64 `json:"TxCompressed,omitempty"`
		TxDropped         int64 `json:"TxDropped,omitempty"`
		TxErrors          int64 `json:"TxErrors,omitempty"`
		TxFifoErrors      int64 `json:"TxFifoErrors,omitempty"`
		TxHeartbeatErrors int64 `json:"TxHeartbeatErrors,omitempty"`
		TxPackets         int64 `json:"TxPackets,omitempty"`
		TxWindowErrors    int64 `json:"TxWindowErrors,omitempty"`
		Start             int64 `json:"Start,omitempty"`
		Last              int64 `json:"Last,omitempty"`
		*/

		//if len(counters) > 0 {
		//sfa.Graph.Lock()
		//defer sfa.Graph.Unlock()
		//sfa.Graph.AddMetadata(sfa.Node, "Sflow-Counters", counters)
		//}
		//graph.Metadata = append(graph.Metadata, counters)
		//	logging.GetLogger().Infof("counters <- Sample = %s", sample)
		//	counters = append(counters, sample)
		//
		//logging.GetLogger().Infof("counters[after append]", counters)
		//sfa.GraphNode.Matadata = append(sfa.GraphNode.Metadata, counters)
		//logging.GetLogger().Infof("Metadata", sfa.GraphNode.Metadata)
		// iterate over a set of Packets as a sample contains multiple  -- added test by Darshan
		// records each generating Packets.
	}

}

func (sfa *Agent) start() error {
	sfa.Lock()
	addr := net.UDPAddr{
		Port: sfa.Port,
		IP:   net.ParseIP(sfa.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		logging.GetLogger().Errorf("Unable to listen on port %d: %s", sfa.Port, err.Error())
		sfa.Unlock()
		return err
	}
	sfa.Conn = conn
	sfa.Unlock()

	sfa.FlowTable.Start()
	defer sfa.FlowTable.Stop()

	sfa.feedFlowTable()

	return nil
}

// Start the SFlow probe agent
func (sfa *Agent) Start() {
	go sfa.start()
}

// Stop the SFlow probe agent
func (sfa *Agent) Stop() {
	sfa.Lock()
	defer sfa.Unlock()

	if sfa.Conn != nil {
		sfa.Conn.Close()
	}
}

// NewAgent creates a new sFlow agent which will populate the given flowtable
func NewAgent(u string, a *common.ServiceAddress, ft *flow.Table, bpfFilter string, headerSize uint32, n *graph.Node, Graph *graph.Graph) *Agent {
	if headerSize == 0 {
		headerSize = flow.DefaultCaptureLength
	}

	return &Agent{
		UUID:       u,
		Addr:       a.Addr,
		Port:       a.Port,
		FlowTable:  ft,
		BPFFilter:  bpfFilter,
		HeaderSize: headerSize,
		Node:       n,
		Graph:      Graph,
	}
}

func (a *AgentAllocator) release(uuid string) {
	for i, agent := range a.agents {
		if uuid == agent.UUID {
			agent.Stop()
			a.portAllocator.Release(agent.Port)
			a.agents = append(a.agents[:i], a.agents[i+1:]...)

			break
		}
	}
}

// Release a sFlow agent
func (a *AgentAllocator) Release(uuid string) {
	a.Lock()
	defer a.Unlock()

	a.release(uuid)
}

// ReleaseAll sFlow agents
func (a *AgentAllocator) ReleaseAll() {
	a.Lock()
	defer a.Unlock()

	for _, agent := range a.agents {
		a.release(agent.UUID)
	}
}

// Alloc allocates a new sFlow agent
func (a *AgentAllocator) Alloc(uuid string, ft *flow.Table, bpfFilter string, headerSize uint32, addr *common.ServiceAddress, n *graph.Node, Graph *graph.Graph) (agent *Agent, _ error) {
	a.Lock()
	defer a.Unlock()
	logging.GetLogger().Infof("Node = %s", n)

	// check if there is an already allocated agent for this uuid
	for _, agent := range a.agents {
		if uuid == agent.UUID {
			return agent, ErrAgentAlreadyAllocated
		}
	}
	logging.GetLogger().Infof("Node = %s", n)

	// get port, if port is not given by user.
	var err error
	if addr.Port <= 0 {
		if addr.Port, err = a.portAllocator.Allocate(); addr.Port <= 0 {
			return nil, errors.New("failed to allocate sflow port: " + err.Error())
		}
	}
	logging.GetLogger().Infof("Node = %s", n)
	s := NewAgent(uuid, addr, ft, bpfFilter, headerSize, n, Graph)

	a.agents = append(a.agents, s)

	s.Start()
	return s, nil
}

// NewAgentAllocator creates a new sFlow agent allocator
func NewAgentAllocator() (*AgentAllocator, error) {
	min := config.GetInt("sflow.port_min")
	max := config.GetInt("sflow.port_max")

	portAllocator, err := common.NewPortAllocator(min, max)
	if err != nil {
		return nil, err
	}

	return &AgentAllocator{portAllocator: portAllocator}, nil
}
