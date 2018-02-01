package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/ioutil"
	"sync"
)

type Cache struct {
	folder string
	hash   hash.Hash
	// TODO put []byte and *sync.Mutex together into one struct to be sure, that
	// there's always a value when there's a mutex and vise versa
	knownValues map[string][]byte
	mutexes     map[string]*sync.Mutex
	mutex       *sync.Mutex
}

func CreateCache(path string) (*Cache, error) {
	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		Error.Printf("Error opening cache folder '%s':\n", path)
		return nil, err
	}

	values := make(map[string][]byte, 0)
	mutexes := make(map[string]*sync.Mutex, 0)

	// Go through every file an save its name in the map. The content of the file
	// is loaded when needed.
	for _, info := range fileInfos {
		if !info.IsDir() {
			values[info.Name()] = nil
		}
	}

	hash := sha256.New()

	mutex := &sync.Mutex{}

	cache := &Cache{
		folder:      path,
		hash:        hash,
		knownValues: values,
		mutexes:     mutexes,
		mutex:       mutex,
	}

	return cache, nil
}

func (c *Cache) has(key string) bool {
	hashValue := calcHash(key)

	_, ok := c.knownValues[hashValue]

	return ok
}

func (c *Cache) get(key string) ([]byte, error) {
	hashValue := calcHash(key)

	// Try to get content. Error if not found.
	content, ok := c.knownValues[hashValue]
	if !ok {
		Debug.Printf("Cache doen't know key '%s'", hashValue)
		return nil, errors.New(fmt.Sprintf("Key '%s' is not known to cache", hashValue))
	}

	mutex := c.ensureMutex(hashValue)

	Debug.Printf("Cache has key '%s'", hashValue)

	// Key is known, but not loaded into RAM
	if content == nil {
		Debug.Printf("Cache has content for '%s' already loaded", hashValue)

		mutex.Lock()

		content, err := ioutil.ReadFile(c.folder + hashValue)
		if err != nil {
			Error.Printf("Error reading cached file '%s'", hashValue)
			return nil, err
		}

		c.knownValues[hashValue] = content

		mutex.Unlock()
	}

	return content, nil
}

func (c *Cache) put(key string, content []byte) error {
	hashValue := calcHash(key)

	mutex := c.ensureMutex(hashValue)

	mutex.Lock()
	defer mutex.Unlock()

	err := ioutil.WriteFile(c.folder+hashValue, content, 0644)

	// Make sure, that the RAM-cache only holds values we were able to write.
	// This is a decision to prevent a false impression of the cache: If the
	// write fails, the cache isn't working correctly, which should be fixed by
	// the user of this cache.
	if err == nil {
		Debug.Printf("Cache wrote content into '%s'", hashValue)
		c.knownValues[hashValue] = content
	} else {
		//TODO remove mutex or is this not neccesarry?
	}

	return err
}

// ensureMutex returns the mutex mapped by the given hashValue. If the mutex
// does not exist, a new one will be created.
func (c *Cache) ensureMutex(hashValue string) *sync.Mutex {
	c.mutex.Lock()
	mutex, hasMutex := c.mutexes[hashValue]
	// Mutex is not known
	if !hasMutex {
		Debug.Printf("Cache doen't know mutex for key '%s'", hashValue)

		mutex = &sync.Mutex{}
		c.mutexes[hashValue] = mutex
	}
	c.mutex.Unlock()

	return mutex
}

func calcHash(data string) string {
	sha := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sha[:])
}
