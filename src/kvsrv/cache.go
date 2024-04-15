package kvsrv

import (
    "container/list"
    "github.com/google/uuid"
)

type Cache struct {
    capacity int
    ll       *list.List
    cache    map[uuid.UUID]*list.Element
}

type entry struct {
    key   uuid.UUID
}

func NewCache(capacity int) *Cache {
    return &Cache{
        capacity: capacity,
        ll:       list.New(),
        cache:    make(map[uuid.UUID]*list.Element),
    }
}

func (c *Cache) Add(key uuid.UUID) {
    if _, ok := c.cache[key]; ok {
        return
    }

    e := c.ll.PushFront(&entry{key})
    c.cache[key] = e
    if c.capacity != 0 && c.ll.Len() > c.capacity {
        c.RemoveOldest()
    }
}

func (c *Cache) RemoveOldest() {
    if e := c.ll.Back(); e != nil {
        c.ll.Remove(e)
        kv := e.Value.(*entry)
        delete(c.cache, kv.key)
    }
}

func (c *Cache) Contains(key uuid.UUID) bool {
    _, ok := c.cache[key]
    return ok
}