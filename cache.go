package fetch

import (
	"container/heap"
	"io"
	"io/fs"
	"log"
	"os"
	"sync"
)

type CacheFS struct {
	sender chan<- Open
}
type Open struct {
	name string
	recv chan<- OpenResp
}
type OpenResp struct {
	f   fs.File
	err error
}

func NewCacheFS() CacheFS {
	sender := make(chan Open, 10)
	return CacheFS{sender: sender}
}

func (c *CacheFS) Open(name string) (fs.File, error) {
	respChan := make(chan OpenResp, 1)
	c.sender <- Open{name: name, recv: respChan}
	resp := <-respChan
	return resp.f, resp.err
}

type Cache struct {
	recv     <-chan Open
	fMap     map[string]CacheFileMeta
	pq       PQ
	clock    int
	origin   Origin
	capacity int
	used     int
	root     string
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
