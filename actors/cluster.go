package actors

import (
	"context"
	"crypto/sha256"
	"math/big"
	"sync"

	actorsv1 "github.com/super-flat/actors/pb/actors"
	parti "github.com/super-flat/parti/cluster"
	"github.com/super-flat/parti/cluster/raftwrapper/discovery"
	partiv1 "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Cluster implements a clustered actor system across many nodes
type Cluster struct {
	partitionsManager *PartitionsManager
	partiCluster      *parti.Cluster
	partitioner       Partitioner
}

// NewCluster returns a new clustered actor system
func NewCluster(raftPort uint16, discoveryPort uint16, partitionCount uint32, discoveryService discovery.Discovery) *Cluster {
	partitionsManager := NewPartitionsManager()

	pCluster := parti.NewCluster(
		raftPort,
		discoveryPort,
		partitionsManager,
		partitionCount,
		discoveryService,
	)

	partitioner := NewHashModPartitioner(partitionCount)

	return &Cluster{
		partitionsManager: partitionsManager,
		partiCluster:      pCluster,
		partitioner:       partitioner,
	}
}

// Send a message to an actor in the cluster, which will forward to remote
// nodes as needed
func (c *Cluster) Send(ctx context.Context, actorID string, message proto.Message) (proto.Message, error) {
	// wrap msg in any
	anyMsg, err := anypb.New(message)
	if err != nil {
		return nil, err
	}
	// pack envelope
	envelope := &actorsv1.Envelope{
		ActorId: actorID,
		Value:   anyMsg,
	}
	// compute partition
	partitionID := c.partitioner.Get(actorID)
	// make parti send request
	anyEnvelope, err := anypb.New(envelope)
	if err != nil {
		return nil, err
	}
	sendRequest := &partiv1.SendRequest{
		PartitionId: partitionID,
		MessageId:   "",
		Message:     anyEnvelope,
	}
	// send to parti
	resp, err := c.partiCluster.Send(ctx, sendRequest)
	if err != nil {
		return nil, err
	}
	// unpack response
	return resp.GetResponse().UnmarshalNew()
}

// PartitionsManager manages many actor dispatchers
type PartitionsManager struct {
	// partiCluster *parti.Cluster
	dispatchers  map[uint32]*Dispatcher
	mtx          *sync.Mutex
	actorFactory ActorFactory
}

// NewPartitionsManager returns a new PartitionsManager
func NewPartitionsManager() *PartitionsManager {
	cluster := &PartitionsManager{
		dispatchers: make(map[uint32]*Dispatcher),
		mtx:         &sync.Mutex{},
	}

	return cluster
}

// Handle handles a message locally
func (c *PartitionsManager) Handle(ctx context.Context, partitionID uint32, msg *anypb.Any) (*anypb.Any, error) {
	// unpack message
	envelope := &actorsv1.Envelope{}
	if err := msg.UnmarshalTo(envelope); err != nil {
		return nil, err
	}
	innerMsg, err := envelope.GetValue().UnmarshalNew()
	if err != nil {
		return nil, err
	}
	// get or create partition
	c.mtx.Lock()
	partition, partitionExists := c.dispatchers[partitionID]
	if !partitionExists {
		partition = NewActorDispatcher(c.actorFactory)
		c.dispatchers[partitionID] = partition
	}
	c.mtx.Unlock()
	// send message to actor
	response, err := partition.Send(ctx, envelope.GetActorId(), innerMsg)
	if err != nil {
		return nil, err
	}
	// wrap and respond to caller
	responseAny, err := anypb.New(response)
	if err != nil {
		return nil, err
	}
	return responseAny, nil
}

// ShutdownPartition blocks until a partition is fully shut down on this node
func (c *PartitionsManager) ShutdownPartition(ctx context.Context, partitionID uint32) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if partition, exists := c.dispatchers[partitionID]; exists {
		partition.Shutdown()
		delete(c.dispatchers, partitionID)
	}
	return nil
}

// Ensure that PartitionsManager implements parti's Handler interface
var _ parti.Handler = &PartitionsManager{}

type Partitioner interface {
	// Get returns the partition for a given actorID
	Get(actorID string) (partition uint32)
}

// HashModPartitioner implements a Partitioner that computes partition by
// hashing the actor ID and computing the modulus of the number of partitions
type HashModPartitioner struct {
	numPartitions uint32
}

// NewHashModPartitioner returns a HashModPartitioner
func NewHashModPartitioner(numPartitions uint32) *HashModPartitioner {
	return &HashModPartitioner{numPartitions: numPartitions}
}

// Get returns the partition for a given actorID
func (d HashModPartitioner) Get(actorID string) uint32 {
	hash := sha256.Sum256([]byte(actorID))
	intHash := new(big.Int)
	intHash.SetBytes(hash[:])
	partitionCount := big.NewInt(int64(d.numPartitions))
	intHash = intHash.Mod(intHash, partitionCount)
	return uint32(intHash.Int64())
}
