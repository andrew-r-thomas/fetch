package fetch

import (
	"container/heap"
	"io"
	"io/fs"
	"log"
	"os"
	"sync"
)

// PERF: ain't no way this phatty lock is the right way to do this
type Cache struct {
	lock     sync.RWMutex
	m        map[string]CacheLine
	accesses chan *PQItem

	capacity int
	used     int
	clock    int
	pqueue   PQ

	root string
}
type CacheLine struct {
	f  *os.File
	qI *PQItem
}

func (c *Cache) Get(path string) (*os.File, bool) {
	c.lock.RLock()
	cl, ok := c.m[path]
	if ok {
		select {
		case c.accesses <- cl.qI:
		default:
			// accesses was full, time to flush
			// PERF: track how much this happens, this is a big perf hit
			c.lock.RUnlock()
			c.lock.Lock()
			for qi := range c.accesses {
				qi.freq += 1
				qi.priority = c.clock + (qi.freq / qi.size)
				heap.Fix(&c.pqueue, qi.qId)
			}
			c.lock.Unlock()
			c.lock.RLock()
			c.accesses <- cl.qI
		}
	}
	return cl.f, ok
}
func (c *Cache) Return(ref **os.File) {
	*ref = nil
	c.lock.RUnlock()
}
func (c *Cache) Put(path string, size int, r io.Reader) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// fix up queue with accesses
	for qi := range c.accesses {
		qi.freq += 1
		qi.priority = c.clock + (qi.freq / qi.size)
		heap.Fix(&c.pqueue, qi.qId)
	}

	// evict until there is room
	for c.used+size > c.capacity {

	}

	// add to map, queue, and make file etc
	f, err := os.Create(c.root + "/" + path)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	c.m[path] = f

	return nil
}

type CacheFileMeta struct {
	cf   *CacheFile
	qi   *PQItem
	freq int
	size int
}
type CacheFile struct {
	lock sync.RWMutex
	f    *os.File
}

func (cf *CacheFile) Read(buf []byte) (int, error) {
	cf.lock.RLock()
	defer cf.lock.RUnlock()
	return cf.f.Read(buf)
}
func (cf *CacheFile) Stat() (fs.FileInfo, error) {
	cf.lock.RLock()
	defer cf.lock.RUnlock()
	return cf.f.Stat()
}
func (cf *CacheFile) Close() error {
	cf.lock.RUnlock()
	return nil
}

func NewCache(capacity int, recv <-chan Open, origin Origin, root string) Cache {
	return Cache{
		recv:     recv,
		capacity: capacity,
		clock:    0,
		fMap:     make(map[string]CacheFileMeta),
		pq:       PQ{},
		origin:   origin,
		root:     root,
		used:     0,
	}
}

func (c *Cache) Start() {
	for open := range c.recv {
		if cfm, ok := c.fMap[open.name]; ok {
			// add access to pq
			cfm.freq += 1
			cfm.qi.priority = c.clock + (cfm.freq / cfm.size)
			heap.Fix(&c.pq, cfm.qi.qId)

			// hand out
			cfm.cf.lock.RLock()
			open.recv <- OpenResp{f: cfm.cf, err: nil}
		} else {
			// file not in cache

			// check origin if it exists
			// -> if not, send open error
			size, reader, err := c.origin.Get(open.name)
			if err != nil {
				open.recv <- OpenResp{f: nil, err: err}
				continue
			}

			// origin check should return the size,
			// if we don't have room,
			// -> pop files until we do,
			//    making sure that we dont
			//    throw away locked files,
			//    need to remove from map and stuff also
			popped := []*PQItem{}
			for c.used+size > c.capacity {
				// TODO:
				top := c.pq.Pop().(*PQItem)
				cf := c.fMap[top.name]
				if cf.cf.lock.TryLock() {
					cf.cf.Close()
					err = os.Remove(c.root + "/" + top.name)
					if err != nil {
						log.Fatalf("error removing file: %v\n", err)
					}
					c.used -= cf.size
					c.clock = cf.qi.priority
					delete(c.fMap, top.name)
				} else {
					// save the popped so that you can add them back to the queue
					popped = append(popped, top)
				}
			}
			for _, p := range popped {
				c.pq.Push(p)
			}

			// write origin file into local file
			f, err := os.Create(c.root + "/" + open.name)
			if err != nil {
				open.recv <- OpenResp{f: nil, err: err}
				continue
			}
			_, err = io.Copy(f, reader)
			if err != nil {
				open.recv <- OpenResp{f: nil, err: err}
				continue
			}

			// add handle to heap, map etc
			c.used += size
			newcf := CacheFile{
				lock: sync.RWMutex{},
				f:    f,
			}
			p := c.clock + (1 / size)
			newpqi := PQItem{name: open.name, priority: p}
			c.pq.Push(&newpqi)
			newcfm := CacheFileMeta{
				cf:   &newcf,
				qi:   &newpqi,
				freq: 1,
				size: size,
			}
			c.fMap[open.name] = newcfm
		}
	}
}
