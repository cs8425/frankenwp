package cache

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync"
	"go.uber.org/zap"
)

type Store struct {
	loc      string
	ttl      int
	logger   *zap.Logger
	memCache atomic.Value // *xsync.MapOf[string, *MemCacheItem]

	// memCache map[string]*MemCacheItem
}

type MemCacheItem struct {
	content   map[int]*string
	value     string
	timestamp int64
}

const (
	CACHE_DIR = "sidekick-cache"
)

func NewStore(loc string, ttl int, logger *zap.Logger) *Store {
	os.MkdirAll(loc+"/"+CACHE_DIR, os.ModePerm)
	memCache := xsync.NewMapOf[*MemCacheItem]()
	d := &Store{
		loc:    loc,
		ttl:    ttl,
		logger: logger,
	}
	d.memCache.Store(memCache)

	// // Load cache from disk
	files, err := os.ReadDir(loc + "/" + CACHE_DIR)
	if err == nil {
		for _, file := range files {
			if file.IsDir() {
				filename := file.Name()
				pageFiles, err := os.ReadDir(loc + "/" + CACHE_DIR + "/" + filename)
				if err != nil {
					continue
				}

				// first time, should not have existing value
				cacheItem, _ := memCache.LoadOrStore(filename, &MemCacheItem{
					content:   make(map[int]*string),
					value:     "",
					timestamp: time.Now().Unix(),
				})

				for idx, pageFile := range pageFiles {
					if !pageFile.IsDir() {
						value, err := os.ReadFile(loc + "/" + CACHE_DIR + "/" + file.Name() + "/" + pageFile.Name())

						if err != nil {
							continue
						}
						newValue := string(value)
						cacheItem.content[idx] = &newValue
						cacheItem.value += newValue
					}
				}
			}
		}
	}

	return d
}

func (d *Store) getMemCache() *xsync.MapOf[string, *MemCacheItem] {
	memCache, ok := d.memCache.Load().(*xsync.MapOf[string, *MemCacheItem])
	if !ok {
		return nil
	}
	return memCache
}

func (d *Store) Get(key string) ([]byte, error) {
	key = strings.ReplaceAll(key, "/", "+")
	d.logger.Debug("Getting key from cache", zap.String("key", key))

	memCache := d.getMemCache()
	cacheItem, ok := memCache.Load(key)
	if ok {
		d.logger.Debug("Pulled key from memory", zap.String("key", key))

		// TODO: fix racing on cacheItem
		if time.Now().Unix()-cacheItem.timestamp > int64(d.ttl) {
			d.logger.Debug("Cache expired", zap.String("key", key))
			// TODO: fix racing when purge running and setting new value with same key
			go d.Purge(key)
			return nil, errors.New("Cache expired")
		}

		d.logger.Debug("Cache hit", zap.String("key", key))
		// TODO: fix racing on cacheItem
		return []byte(cacheItem.value), nil
	}

	// TODO: fix racing when new value is writting and someone read it at same time
	// eg: already wrote 5/10 files but not yet finished(10/10), and someone can only read these 5 files, result as only get partial data
	// load files in directory
	files, err := os.ReadDir(d.loc + "/" + CACHE_DIR + "/" + key)
	if err != nil {
		return nil, errors.New("Key not found in cache")
	}

	content := ""

	for _, file := range files {
		if !file.IsDir() {
			value, err := os.ReadFile(d.loc + "/" + CACHE_DIR + "/" + key + "/" + file.Name())
			if err != nil {
				return nil, errors.New("Key not found in cache")
			}

			content += string(value)
		}
	}

	d.logger.Debug("Cache hit", zap.String("key", key))
	d.logger.Debug("Pulled key from disk", zap.String("key", key))

	return []byte(content), nil
}

// TODO: why we need index here?
func (d *Store) Set(key string, idx int, value []byte) error {
	key = strings.ReplaceAll(key, "/", "+")

	memCache := d.getMemCache()
	cacheItem, _ := memCache.LoadOrStore(key, &MemCacheItem{
		content:   make(map[int]*string),
		value:     "",
		timestamp: time.Now().Unix(),
	})

	d.logger.Debug("-----------------------------------")
	d.logger.Debug("Setting key in cache", zap.String("key", key))
	d.logger.Debug("Index", zap.Int("index", idx))
	newValue := string(value)

	// TODO: fix racing on cacheItem
	if idx == 0 {
		cacheItem.timestamp = time.Now().Unix()
	}

	// TODO: fix racing on cacheItem
	cacheItem.value += newValue

	// create page directory
	os.MkdirAll(d.loc+"/"+CACHE_DIR+"/"+key, os.ModePerm)
	err := os.WriteFile(d.loc+"/"+CACHE_DIR+"/"+key+"/"+strconv.Itoa(idx), value, os.ModePerm)

	if err != nil {
		d.logger.Error("Error writing to cache", zap.Error(err))
	}

	return nil
}

func (d *Store) Purge(key string) {
	key = strings.ReplaceAll(key, "/", "+")
	d.logger.Debug("Removing key from cache", zap.String("key", key))

	memCache := d.getMemCache()
	memCache.Delete("br::" + key)
	memCache.Delete("gzip::" + key)
	memCache.Delete("none::" + key)

	os.RemoveAll(d.loc + "/" + CACHE_DIR + "/br::" + key)
	os.RemoveAll(d.loc + "/" + CACHE_DIR + "/gzip::" + key)
	os.RemoveAll(d.loc + "/" + CACHE_DIR + "/none::" + key)
}

func (d *Store) Flush() error {
	d.memCache.Store(xsync.NewMapOf[*MemCacheItem]())
	err := os.RemoveAll(d.loc + "/" + CACHE_DIR)

	if err == nil {
		os.MkdirAll(d.loc+"/"+CACHE_DIR, os.ModePerm)
	} else {
		d.logger.Error("Error flushing cache", zap.Error(err))
	}

	return err
}

func (d *Store) List() map[string][]string {
	memCache := d.getMemCache()
	list := make(map[string][]string)
	list["mem"] = make([]string, 0, memCache.Size())

	memCache.Range(func(key string, value *MemCacheItem) bool {
		list["mem"] = append(list["mem"], key)
		return true
	})

	files, err := os.ReadDir(d.loc + "/" + CACHE_DIR)
	list["disk"] = make([]string, 0)

	if err == nil {
		for _, file := range files {
			if !file.IsDir() {
				list["disk"] = append(list["disk"], file.Name())
			}
		}
	}

	return list
}
