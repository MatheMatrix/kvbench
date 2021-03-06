package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/kvbench"
	"github.com/smallnest/log"
)

var (
	n     = flag.Int("n", 10000, "count")
	c     = flag.Int("c", runtime.GOMAXPROCS(-1), "concurrent goroutines")
	size  = flag.Int("size", 256, "data size")
	fsync = flag.Bool("fsync", false, "fsync")
	s     = flag.String("s", "map", "store type")
)

func main() {
	flag.Parse()

	fmt.Printf("n=%d, c=%d\n", *n, *c)

	var memory bool
	var path string
	if strings.HasSuffix(*s, "/memory") {
		memory = true
		path = ":memory:"
		*s = strings.TrimSuffix(*s, "/memory")
	}

	store, path, err := getStore(*s, *fsync, path)
	if err != nil {
		panic(err)
	}
	if memory {
		defer os.RemoveAll(path)
	}

	name := *s
	if memory {
		name = name + "/memory"
	}
	if *fsync {
		name = name + "/fsync"
	} else {
		name = name + "/nofsync"
	}

	data := make([]byte, *size)
	numPerG := *n / (*c)

	// test set
	{
		var wg sync.WaitGroup
		wg.Add(*c)

		start := time.Now()
		for j := 0; j < *c; j++ {
			index := uint64(j)
			go func() {
				i := index
				for k := 0; k < numPerG; k++ {
					store.Set(genKey(i), data)
					i += index
				}
				wg.Done()
			}()
		}
		wg.Wait()
		dur := time.Since(start)
		d := int64(dur)
		fmt.Printf("%s set rate: %d op/s, mean: %d ns, took: %d s\n", name, int64(*n)*1e6/(d/1e3), d/int64((*n)*(*c)), int(dur.Seconds()))
	}

	// test get
	{
		var wg sync.WaitGroup
		wg.Add(*c)

		start := time.Now()
		for j := 0; j < *c; j++ {
			index := uint64(j)
			go func() {
				i := index
				for k := 0; k < numPerG; k++ {
					store.Get(genKey(i))
					i += index
				}
				wg.Done()
			}()
		}
		wg.Wait()
		dur := time.Since(start)
		d := int64(dur)
		fmt.Printf("%s get rate: %d op/s, mean: %d ns, took: %d s\n", name, int64(*n)*1e6/(d/1e3), d/int64((*n)*(*c)), int(dur.Seconds()))
	}

	// test multiple get/one set
	{
		var wg sync.WaitGroup
		wg.Add(*c)

		ch := make(chan struct{})

		var setCount uint64

		start := time.Now()
		go func() {
			i := uint64(0)
			for {
				select {
				case <-ch:
					return
				default:
					store.Set(genKey(i), data)
					setCount++
					i++
				}
			}
		}()
		for j := 0; j < *c; j++ {
			index := uint64(j)
			go func() {

				i := index
				for k := 0; k < numPerG; k++ {
					store.Get(genKey(i))
					i += index
				}
				wg.Done()
			}()
		}
		wg.Wait()
		close(ch)
		dur := time.Since(start)
		d := int64(dur)

		if setCount == 0 {
			fmt.Printf("%s setmixed rate: -1 op/s, mean: -1 ns, took: %d s\n", name, int(dur.Seconds()))
		} else {
			fmt.Printf("%s setmixed rate: %d op/s, mean: %d ns, took: %d s\n", name, int64(setCount)*1e6/(d/1e3), d/int64(setCount), int(dur.Seconds()))
		}
		fmt.Printf("%s getmixed rate: %d op/s, mean: %d ns, took: %d s\n", name, int64(*n)*1e6/(d/1e3), d/int64((*n)*(*c)), int(dur.Seconds()))
	}

	// test del
	{
		var wg sync.WaitGroup
		wg.Add(*c)

		start := time.Now()
		for j := 0; j < *c; j++ {
			index := uint64(j)
			go func() {
				i := index
				for k := 0; k < numPerG; k++ {
					store.Del(genKey(i))
					i += index
				}
				wg.Done()
			}()
		}
		wg.Wait()
		dur := time.Since(start)
		d := int64(dur)
		fmt.Printf("%s del rate: %d op/s, mean: %d ns, took: %d s\n", name, int64(*n)*1e6/(d/1e3), d/int64((*n)*(*c)), int(dur.Seconds()))
	}
}

func genKey(i uint64) []byte {
	r := make([]byte, 9)
	r[0] = 'k'
	binary.BigEndian.PutUint64(r[1:], i)
	return r
}

func getStore(s string, fsync bool, path string) (kvbench.Store, string, error) {
	var store kvbench.Store
	var err error
	switch s {
	default:
		err = fmt.Errorf("unknown store type: %v", s)
	case "map":
		if path == "" {
			path = "map.db"
		}
		store, err = kvbench.NewMapStore(path, fsync)
	case "btree":
		if path == "" {
			path = "btree.db"
		}
		store, err = kvbench.NewBTreeStore(path, fsync)
	case "bolt":
		if path == "" {
			path = "bolt.db"
		}
		store, err = kvbench.NewBoltStore(path, fsync)
	case "bbolt":
		if path == "" {
			path = "bbolt.db"
		}
		store, err = kvbench.NewBboltStore(path, fsync)
	case "leveldb":
		if path == "" {
			path = "leveldb.db"
		}
		store, err = kvbench.NewLevelDBStore(path, fsync)
	case "kv":
		log.Warningf("kv store is unstable")
		if path == "" {
			path = "kv.db"
		}
		store, err = kvbench.NewKVStore(path, fsync)
	case "badger":
		if path == "" {
			path = "badger.db"
		}
		store, err = kvbench.NewBadgerStore(path, fsync)
	case "buntdb":
		if path == "" {
			path = "buntdb.db"
		}
		store, err = kvbench.NewBuntdbStore(path, fsync)
	case "rocksdb":
		if path == "" {
			path = "rocksdb.db"
		}
		store, err = kvbench.NewRocksdbStore(path, fsync)
	case "pebble":
		if path == "" {
			path = "pebble.db"
		}
		store, err = kvbench.NewRocksdbStore(path, fsync)
	}

	return store, path, err
}
