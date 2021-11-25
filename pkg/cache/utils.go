package cache

import (
	"hash/fnv"
)

var hasher = fnv.New64a()

func getIndexKey(key string) uint64 {
	hasher.Reset()
	hasher.Write([]byte(key))
	return hasher.Sum64()
}
