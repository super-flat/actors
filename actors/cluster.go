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

type Cluster struct {
	partitions     *PartitionsManager
	partiCluster   *parti.Cluster
	partitionCount uint32
}

func NewCluster(raftPort uint16, discoveryPort uint16, partitionCount uint32, discoveryService discovery.Discovery) *Cluster {
	partitionsManager := NewPartitionsManager()

	pCluster := parti.NewCluster(
		raftPort,
		discoveryPort,
		partitionsManager,
		partitionCount,
		discoveryService,
	)

	return &Cluster{
		partitions:     partitionsManager,
		partiCluster:   pCluster,
		partitionCount: partitionCount,
	}
}

// hash a string value into a uint32
// TODO: make this safer / smarter
func (c Cluster) hash(val string) uint32 {
	hash := sha256.Sum256([]byte(val))
	intHash := new(big.Int)
	intHash.SetBytes(hash[:])
	partitionCount := big.NewInt(int64(c.partitionCount))
	intHash = intHash.Mod(intHash, partitionCount)
	return uint32(intHash.Int64())
}

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
	partitionID := c.hash(actorID)
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

type PartitionsManager struct {
	// partiCluster *parti.Cluster
	partitions   map[uint32]*Dispatcher
	mtx          *sync.Mutex
	actorFactory ActorFactory
}

func NewPartitionsManager() *PartitionsManager {
	cluster := &PartitionsManager{
		partitions: make(map[uint32]*Dispatcher),
		mtx:        &sync.Mutex{},
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
	partition, partitionExists := c.partitions[partitionID]
	if !partitionExists {
		partition = NewActorDispatcher(c.actorFactory)
		c.partitions[partitionID] = partition
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
	if partition, exists := c.partitions[partitionID]; exists {
		partition.Shutdown()
		delete(c.partitions, partitionID)
	}
	return nil
}

var _ parti.Handler = &PartitionsManager{}
