package cache

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync"
	"go.uber.org/zap"
)

var (
	ErrCacheExpired  = errors.New("cache expired")
	ErrCacheNotFound = errors.New("key not found in cache")
)

type Store struct {
	loc      string
	ttl      int
	logger   *zap.Logger
	memCache atomic.Value // *xsync.MapOf[string, *MemCacheItem]

	// memCache map[string]*MemCacheItem
}

type MemCacheItem struct {
	timestamp int64

	stateCode       int
	contentEncoding string
	header          [][]string
	value           []byte
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
	/*files, err := os.ReadDir(loc + "/" + CACHE_DIR)
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
					value:     nil,
					timestamp: time.Now().Unix(),
				})

				// TODO: load header, stateCode, timestamp
				for _, pageFile := range pageFiles {
					if !pageFile.IsDir() {
						value, err := os.ReadFile(loc + "/" + CACHE_DIR + "/" + file.Name() + "/" + pageFile.Name())

						if err != nil {
							continue
						}
						cacheItem.value = append(cacheItem.value, value...)
					}
				}
			}
		}
	}*/

	return d
}

func (d *Store) getMemCache() *xsync.MapOf[string, *MemCacheItem] {
	memCache, ok := d.memCache.Load().(*xsync.MapOf[string, *MemCacheItem])
	if !ok {
		return nil
	}
	return memCache
}

func (d *Store) Get(key string) ([]byte, int, error) {
	key = strings.ReplaceAll(key, "/", "+")
	d.logger.Debug("Getting key from cache", zap.String("key", key))

	memCache := d.getMemCache()
	cacheItem, ok := memCache.Load(key)
	if ok {
		d.logger.Debug("Pulled key from memory", zap.String("key", key))

		if time.Now().Unix()-cacheItem.timestamp > int64(d.ttl) {
			d.logger.Debug("Cache expired", zap.String("key", key))
			// TODO: fix racing when purge running and setting new value with same key
			go d.Purge(key)
			return nil, 0, ErrCacheExpired
		}

		d.logger.Debug("Cache hit", zap.String("key", key))
		return []byte(cacheItem.value), cacheItem.stateCode, nil
	}

	// TODO: finish load from disk
	return nil, 0, ErrCacheNotFound
}

// TODO: why we need index here?
func (d *Store) Set(reqPath string, ct string, cacheKey string, stateCode int, value []byte) error {
	key := d.buildCacheKey(reqPath, ct, cacheKey)
	d.logger.Debug("Cache Key", zap.String("Key", key))

	key = strings.ReplaceAll(key, "/", "+")

	memCache := d.getMemCache()
	_, existed := memCache.LoadAndStore(key, &MemCacheItem{
		stateCode:       stateCode,
		contentEncoding: ct,
		value:           value,
		timestamp:       time.Now().Unix(),
	})

	d.logger.Debug("-----------------------------------")
	d.logger.Debug("Setting key in cache", zap.String("key", key), zap.Bool("replace", existed))

	// create page directory
	basePath := path.Join(d.loc, CACHE_DIR, key)
	os.MkdirAll(basePath, os.ModePerm)
	err := os.WriteFile(path.Join(basePath, "."+ct), value, os.ModePerm)

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

func (d *Store) buildCacheKey(reqPath string, contentEncoding string, cacheKey string) string {
	// cacheKey := contentEncoding + "::" + reqPath
	if len(cacheKey) == 0 {
		return fmt.Sprintf("%v::%v", reqPath, contentEncoding)
	}
	return fmt.Sprintf("%v::%v::cacheKey", reqPath, contentEncoding, cacheKey)
}
