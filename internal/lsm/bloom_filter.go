package lsm

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"math"
)

// BloomFilter implements a space-efficient probabilistic data structure
// that is used to test whether an element is a member of a set
type BloomFilter struct {
	bitArray  []uint64 // Bit array stored as uint64 slices for efficiency
	size      uint64   // Size of bit array
	numHashes int      // Number of hash functions
	numItems  uint64   // Number of items added

	// Hash functions
	hashFuncs []hash.Hash64

	// Statistics
	falsePositiveRate float64
	expectedItems     uint64
}

// NewBloomFilter creates a new Bloom filter with the specified expected number of items
// and false positive rate
func NewBloomFilter(expectedItems uint64, falsePositiveRate float64) *BloomFilter {
	if expectedItems == 0 {
		expectedItems = 1000 // Default fallback
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01 // Default 1%
	}

	// Calculate optimal bit array size and number of hash functions
	size := optimalSize(expectedItems, falsePositiveRate)
	numHashes := optimalNumHashes(size, expectedItems)

	// Ensure size is aligned to uint64 boundaries
	arraySize := (size + 63) / 64

	bf := &BloomFilter{
		bitArray:          make([]uint64, arraySize),
		size:              size,
		numHashes:         numHashes,
		falsePositiveRate: falsePositiveRate,
		expectedItems:     expectedItems,
		hashFuncs:         make([]hash.Hash64, numHashes),
	}

	// Initialize hash functions
	for i := 0; i < numHashes; i++ {
		bf.hashFuncs[i] = fnv.New64a()
	}

	return bf
}

// optimalSize calculates the optimal bit array size for given parameters
// Formula: m = -(n * ln(p)) / (ln(2)^2)
// where n = expected items, p = false positive rate
func optimalSize(expectedItems uint64, falsePositiveRate float64) uint64 {
	ln2Squared := math.Ln2 * math.Ln2
	size := -float64(expectedItems) * math.Log(falsePositiveRate) / ln2Squared
	return uint64(math.Ceil(size))
}

// optimalNumHashes calculates the optimal number of hash functions
// Formula: k = (m/n) * ln(2)
// where m = bit array size, n = expected items
func optimalNumHashes(bitArraySize, expectedItems uint64) int {
	ratio := float64(bitArraySize) / float64(expectedItems)
	numHashes := ratio * math.Ln2
	result := int(math.Round(numHashes))

	// Ensure at least 1 hash function
	if result < 1 {
		return 1
	}

	// Cap at reasonable maximum to avoid excessive computation
	if result > 20 {
		return 20
	}

	return result
}

// Add adds an item to the bloom filter
func (bf *BloomFilter) Add(data []byte) {
	if len(data) == 0 {
		return
	}

	for i := 0; i < bf.numHashes; i++ {
		// Reset hash function
		bf.hashFuncs[i].Reset()

		// Add salt to make each hash function different
		salt := []byte{byte(i)}
		bf.hashFuncs[i].Write(salt)
		bf.hashFuncs[i].Write(data)

		// Get hash value and set corresponding bit
		hashValue := bf.hashFuncs[i].Sum64()
		bitIndex := hashValue % bf.size
		bf.setBit(bitIndex)
	}

	bf.numItems++
}

// MightContain tests whether an item might be in the set
// Returns true if the item might be in the set (could be false positive)
// Returns false if the item is definitely not in the set
func (bf *BloomFilter) MightContain(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for i := 0; i < bf.numHashes; i++ {
		// Reset hash function
		bf.hashFuncs[i].Reset()

		// Add salt to make each hash function different
		salt := []byte{byte(i)}
		bf.hashFuncs[i].Write(salt)
		bf.hashFuncs[i].Write(data)

		// Get hash value and check corresponding bit
		hashValue := bf.hashFuncs[i].Sum64()
		bitIndex := hashValue % bf.size

		if !bf.getBit(bitIndex) {
			return false // Definitely not in set
		}
	}

	return true // Might be in set
}

// setBit sets the bit at the given index
func (bf *BloomFilter) setBit(bitIndex uint64) {
	arrayIndex := bitIndex / 64
	bitOffset := bitIndex % 64

	if arrayIndex < uint64(len(bf.bitArray)) {
		bf.bitArray[arrayIndex] |= (1 << bitOffset)
	}
}

// getBit gets the bit at the given index
func (bf *BloomFilter) getBit(bitIndex uint64) bool {
	arrayIndex := bitIndex / 64
	bitOffset := bitIndex % 64

	if arrayIndex >= uint64(len(bf.bitArray)) {
		return false
	}

	return (bf.bitArray[arrayIndex] & (1 << bitOffset)) != 0
}

// EstimatedFalsePositiveRate calculates the current false positive rate
// based on the number of items added
func (bf *BloomFilter) EstimatedFalsePositiveRate() float64 {
	if bf.numItems == 0 {
		return 0.0
	}

	// Calculate the probability that a bit is still 0
	// P(bit is 0) = (1 - 1/m)^(k*n)
	// where m = bit array size, k = number of hash functions, n = number of items
	probBitIsZero := math.Pow(1.0-1.0/float64(bf.size), float64(bf.numHashes)*float64(bf.numItems))

	// False positive rate = (1 - P(bit is 0))^k
	fpr := math.Pow(1.0-probBitIsZero, float64(bf.numHashes))

	return fpr
}

// Clear resets the bloom filter
func (bf *BloomFilter) Clear() {
	for i := range bf.bitArray {
		bf.bitArray[i] = 0
	}
	bf.numItems = 0
}

// Count returns the number of items added to the filter
func (bf *BloomFilter) Count() uint64 {
	return bf.numItems
}

// Size returns the size of the bit array
func (bf *BloomFilter) Size() uint64 {
	return bf.size
}

// NumHashFunctions returns the number of hash functions used
func (bf *BloomFilter) NumHashFunctions() int {
	return bf.numHashes
}

// ExpectedFalsePositiveRate returns the configured false positive rate
func (bf *BloomFilter) ExpectedFalsePositiveRate() float64 {
	return bf.falsePositiveRate
}

// MemoryUsage returns the memory usage in bytes
func (bf *BloomFilter) MemoryUsage() uint64 {
	// Size of bit array in bytes + overhead for struct fields
	return uint64(len(bf.bitArray)*8) + 64 // Rough estimate for struct overhead
}

// Union combines this bloom filter with another one
// Both filters must have the same parameters (size, hash functions)
func (bf *BloomFilter) Union(other *BloomFilter) error {
	if bf.size != other.size || bf.numHashes != other.numHashes {
		return fmt.Errorf("bloom filters must have same size and number of hash functions")
	}

	for i := range bf.bitArray {
		bf.bitArray[i] |= other.bitArray[i]
	}

	// Update item count (this is an approximation)
	bf.numItems += other.numItems

	return nil
}

// Intersection calculates the intersection of this bloom filter with another
// Both filters must have the same parameters (size, hash functions)
func (bf *BloomFilter) Intersection(other *BloomFilter) error {
	if bf.size != other.size || bf.numHashes != other.numHashes {
		return fmt.Errorf("bloom filters must have same size and number of hash functions")
	}

	for i := range bf.bitArray {
		bf.bitArray[i] &= other.bitArray[i]
	}

	// Item count after intersection is difficult to estimate accurately
	// We'll use the minimum as a conservative estimate
	if other.numItems < bf.numItems {
		bf.numItems = other.numItems
	}

	return nil
}

// GetStats returns statistics about the bloom filter
func (bf *BloomFilter) GetStats() BloomFilterStats {
	return BloomFilterStats{
		Size:             bf.size,
		NumHashFunctions: bf.numHashes,
		NumItems:         bf.numItems,
		ExpectedItems:    bf.expectedItems,
		ExpectedFPR:      bf.falsePositiveRate,
		EstimatedFPR:     bf.EstimatedFalsePositiveRate(),
		MemoryUsage:      bf.MemoryUsage(),
		LoadFactor:       float64(bf.numItems) / float64(bf.expectedItems),
	}
}

// BloomFilterStats holds statistics about a bloom filter
type BloomFilterStats struct {
	Size             uint64  // Size of bit array
	NumHashFunctions int     // Number of hash functions
	NumItems         uint64  // Number of items added
	ExpectedItems    uint64  // Expected number of items
	ExpectedFPR      float64 // Expected false positive rate
	EstimatedFPR     float64 // Current estimated false positive rate
	MemoryUsage      uint64  // Memory usage in bytes
	LoadFactor       float64 // Current load factor (numItems / expectedItems)
}

// Serialize serializes the bloom filter to bytes for persistence
func (bf *BloomFilter) Serialize() []byte {
	// Calculate total size needed
	headerSize := 32 // Space for metadata
	dataSize := len(bf.bitArray) * 8
	totalSize := headerSize + dataSize

	data := make([]byte, totalSize)
	offset := 0

	// Write metadata
	binary.LittleEndian.PutUint64(data[offset:], bf.size)
	offset += 8
	binary.LittleEndian.PutUint64(data[offset:], uint64(bf.numHashes))
	offset += 8
	binary.LittleEndian.PutUint64(data[offset:], bf.numItems)
	offset += 8
	binary.LittleEndian.PutUint64(data[offset:], bf.expectedItems)
	offset += 8

	// Write bit array
	for _, word := range bf.bitArray {
		binary.LittleEndian.PutUint64(data[offset:], word)
		offset += 8
	}

	return data
}

// Deserialize creates a bloom filter from serialized bytes
func DeserializeBloomFilter(data []byte, falsePositiveRate float64) (*BloomFilter, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("invalid serialized bloom filter data")
	}

	offset := 0

	// Read metadata
	size := binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	numHashes := int(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	numItems := binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	expectedItems := binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// Calculate expected bit array size
	arraySize := (size + 63) / 64
	expectedDataSize := int(arraySize * 8)

	if len(data) < 32+expectedDataSize {
		return nil, fmt.Errorf("invalid serialized bloom filter data size")
	}

	// Create bloom filter
	bf := &BloomFilter{
		bitArray:          make([]uint64, arraySize),
		size:              size,
		numHashes:         numHashes,
		numItems:          numItems,
		expectedItems:     expectedItems,
		falsePositiveRate: falsePositiveRate,
		hashFuncs:         make([]hash.Hash64, numHashes),
	}

	// Initialize hash functions
	for i := 0; i < numHashes; i++ {
		bf.hashFuncs[i] = fnv.New64a()
	}

	// Read bit array
	for i := range bf.bitArray {
		bf.bitArray[i] = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
	}

	return bf, nil
}
