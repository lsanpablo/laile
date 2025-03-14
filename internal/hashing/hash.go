// Package hashing provides functions for hashing strings and implementing consistent hashing
package hashing

import (
	"hash/fnv"
	"sort"
)

// HashKey64Bit returns a 64-bit FNV-1a hash of the given string.
func HashKey64Bit(key string) uint64 {
	fnvHasher := fnv.New64a()
	fnvHasher.Write([]byte(key))
	return fnvHasher.Sum64()
}

// GetRangeEnd returns the hash key that marks the exclusive upper bound of the current range in the hash ring.
// It finds the first key in the sorted hashRing that is greater than or equal to hashKey.
// Because the hash ring is circular, if hashKey is greater than all elements in the ring,
// the function wraps around and returns the first element, along with a flag indicating the wrap-around.
func GetRangeEnd(hashKey uint64, hashRing *[]uint64) (uint64, bool) {
	idx := sort.Search(len(*hashRing), func(i int) bool { return (*hashRing)[i] >= hashKey })
	if idx < len(*hashRing) {
		return (*hashRing)[idx], false
	}
	return (*hashRing)[0], true
}
