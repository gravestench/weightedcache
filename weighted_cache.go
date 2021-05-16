package weightedcache

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

type key = string

// New creates a new instance of a WeightedCache
func New(budget int) *WeightedCache {
	return &WeightedCache{lookup: make(map[key]*node), budget: budget}
}

type node struct {
	key
	next   *node
	prev   *node
	value  interface{}
	weight int
}

// WeightedCache stores arbitrary data for fast retrieval
type WeightedCache struct {
	head    *node
	tail    *node
	lookup  map[string]*node
	weight  int
	budget  int
	verbose bool
	mutex   sync.Mutex
	logger  io.Writer
}

// SetVerbose turns on verbose printing (warnings and stuff)
func (c *WeightedCache) SetVerbose(verbose bool) {
	c.verbose = verbose
}

// Weight gets the "weight" of a cache
func (c *WeightedCache) Weight() int {
	return c.weight
}

// Budget gets the memory budget of a cache
func (c *WeightedCache) Budget() int {
	return c.budget
}

const (
	fmtErrEvict = "evicting %s (%d) for %s (%d); spare weight is now %d"
)

// Insert inserts an object into the cache
func (c *WeightedCache) Insert(key string, value interface{}, weight int) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, found := c.lookup[key]; found {
		return errors.New("key already exists in WeightedCache")
	}

	node := &node{
		key:    key,
		value:  value,
		weight: weight,
		next:   c.head,
	}

	if c.head != nil {
		c.head.prev = node
	}

	c.head = node
	if c.tail == nil {
		c.tail = node
	}

	c.lookup[key] = node
	c.weight += node.weight

	for ; c.tail != nil && c.tail != c.head && c.weight > c.budget; c.tail = c.tail.prev {
		c.weight -= c.tail.weight
		c.tail.prev.next = nil

		if c.verbose && c.logger != nil {
			msg := fmt.Sprintf(fmtErrEvict, c.tail.key, c.tail.weight, key, weight, c.budget - c.weight)
			if _, err := c.logger.Write(([]byte)(msg)); err != nil {
				return err
			}
		}

		delete(c.lookup, c.tail.key)
	}

	return nil
}

// Retrieve gets an object out of the cache
func (c *WeightedCache) Retrieve(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	node, found := c.lookup[key]
	if !found {
		return nil, false
	}

	if node != c.head {
		if node.next != nil {
			node.next.prev = node.prev
		}

		if node.prev != nil {
			node.prev.next = node.next
		}

		if node == c.tail {
			c.tail = c.tail.prev
		}

		node.next = c.head
		node.prev = nil

		if c.head != nil {
			c.head.prev = node
		}

		c.head = node
	}

	return node.value, true
}

// Clear removes all cache entries
func (c *WeightedCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.head = nil
	c.tail = nil
	c.lookup = make(map[string]*node)
	c.weight = 0
}
