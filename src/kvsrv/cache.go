package kvsrv

import (
    "container/list"
    "github.com/google/uuid"
)


type Entry struct {
    OpId       uuid.UUID
    Operation  *Operation
}

type LruCache struct {
    MaxCapacity int
    CacheList   *list.List
    CacheMap    map[uuid.UUID]*list.Element
}

func newLruCache(maxCapacity int) *LruCache {
    return &LruCache{
        MaxCapacity: maxCapacity,
        CacheList:   list.New(),
        CacheMap:    make(map[uuid.UUID]*list.Element),
    }
}

func (lc *LruCache) Add(op Operation) {
    if lc.CacheList.Len() >= lc.MaxCapacity {
        delete(lc.CacheMap, lc.CacheList.Back().Value.(*Entry).OpId)
		retentry := (lc.CacheList.Back().Value.(*Entry).Operation)
		OpPool.Put(retentry)
        lc.CacheList.Remove(lc.CacheList.Back())

    }

    entry := &Entry{
        OpId:       op.Opid,
        Operation:  &op,
    }

    element := lc.CacheList.PushFront(entry)
    lc.CacheMap[op.Opid] = element
}

func (lc *LruCache) Get(opId uuid.UUID) (*Operation, bool) {
    if element, ok := lc.CacheMap[opId]; ok {
        return element.Value.(*Entry).Operation, true
    }

    return nil, false
}

func (lc *LruCache) Contain(opId uuid.UUID) bool {
    _, ok := lc.CacheMap[opId]
    return ok
}