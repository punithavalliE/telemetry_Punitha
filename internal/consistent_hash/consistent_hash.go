package consistenthash

import (
	"crypto/sha512"
	"fmt"
	"sort"
)

// ConsistentHash implements a consistent hashing ring with virtual nodes
type ConsistentHash struct {
	ring         map[uint32]string // hash -> broker
	sortedHashes []uint32
	brokers      []string
	virtualNodes int // Number of virtual nodes per broker
}

// NewConsistentHash creates a new consistent hash ring
func NewConsistentHash(brokers []string, virtualNodes int) *ConsistentHash {
	ch := &ConsistentHash{
		ring:         make(map[uint32]string),
		brokers:      make([]string, len(brokers)),
		virtualNodes: virtualNodes,
	}
	copy(ch.brokers, brokers)
	ch.buildRing()
	return ch
}

// buildRing constructs the hash ring with virtual nodes
func (ch *ConsistentHash) buildRing() {
	ch.ring = make(map[uint32]string)
	ch.sortedHashes = []uint32{}

	// Create virtual nodes for each broker
	for _, broker := range ch.brokers {
		for i := 0; i < ch.virtualNodes; i++ {
			virtualNode := fmt.Sprintf("%s:%d", broker, i)
			hash := ch.hash(virtualNode)
			ch.ring[hash] = broker
			ch.sortedHashes = append(ch.sortedHashes, hash)
		}
	}

	// Sort hashes for binary search
	sort.Slice(ch.sortedHashes, func(i, j int) bool {
		return ch.sortedHashes[i] < ch.sortedHashes[j]
	})
}

// hash computes SHA512 hash and returns first 4 bytes as uint32
func (ch *ConsistentHash) hash(key string) uint32 {
	h := sha512.Sum512([]byte(key))
	return uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
}

// GetBroker returns the broker responsible for the given partition
func (ch *ConsistentHash) GetBroker(partition int) string {
	if len(ch.brokers) == 0 {
		return ""
	}

	// Hash the partition ID
	partitionKey := fmt.Sprintf("partition-%d", partition)
	hash := ch.hash(partitionKey)

	// Find the first broker >= hash (clockwise on ring)
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// Wrap around if necessary
	if idx == len(ch.sortedHashes) {
		idx = 0
	}

	return ch.ring[ch.sortedHashes[idx]]
}

// GetBrokerByKey returns the broker responsible for the given key
func (ch *ConsistentHash) GetBrokerByKey(key string) string {
	if len(ch.brokers) == 0 {
		return ""
	}

	hash := ch.hash(key)

	// Find the first broker >= hash (clockwise on ring)
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// Wrap around if necessary
	if idx == len(ch.sortedHashes) {
		idx = 0
	}

	return ch.ring[ch.sortedHashes[idx]]
}

// GetBrokerByTopicPartition returns the broker responsible for the given topic-partition combination
// This allows different partitions of the same topic to be distributed across different brokers
func (ch *ConsistentHash) GetBrokerByTopicPartition(topic string, partition int) string {
	if len(ch.brokers) == 0 {
		return ""
	}

	// Combine topic and partition for better distribution
	topicPartitionKey := fmt.Sprintf("%s-partition-%d", topic, partition)
	hash := ch.hash(topicPartitionKey)

	// Find the first broker >= hash (clockwise on ring)
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// Wrap around if necessary
	if idx == len(ch.sortedHashes) {
		idx = 0
	}

	return ch.ring[ch.sortedHashes[idx]]
}

// AddBroker adds a new broker to the ring with minimal rebalancing
func (ch *ConsistentHash) AddBroker(broker string) {
	// Check if broker already exists
	for _, b := range ch.brokers {
		if b == broker {
			return
		}
	}

	ch.brokers = append(ch.brokers, broker)
	ch.buildRing()
}

// RemoveBroker removes a broker from the ring with minimal rebalancing
func (ch *ConsistentHash) RemoveBroker(broker string) {
	for i, b := range ch.brokers {
		if b == broker {
			ch.brokers = append(ch.brokers[:i], ch.brokers[i+1:]...)
			break
		}
	}
	ch.buildRing()
}

// GetBrokers returns all brokers in the ring
func (ch *ConsistentHash) GetBrokers() []string {
	result := make([]string, len(ch.brokers))
	copy(result, ch.brokers)
	return result
}

// GetBrokerCount returns the number of brokers
func (ch *ConsistentHash) GetBrokerCount() int {
	return len(ch.brokers)
}

// GetPartitionDistribution returns how partitions are distributed across brokers
func (ch *ConsistentHash) GetPartitionDistribution(maxPartitions int) map[string][]int {
	distribution := make(map[string][]int)

	for i := 0; i < maxPartitions; i++ {
		broker := ch.GetBroker(i)
		distribution[broker] = append(distribution[broker], i)
	}

	return distribution
}

// HashPartition returns a partition number for a given key using consistent hashing
func (ch *ConsistentHash) HashPartition(key string, maxPartitions int) int {
	if maxPartitions <= 0 {
		return 0
	}

	hash := ch.hash(key)
	return int(hash) % maxPartitions
}
