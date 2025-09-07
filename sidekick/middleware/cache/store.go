package cache

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

var (
	ErrCacheExpired  = errors.New("cache expired")
	ErrCacheNotFound = errors.New("key not found in cache")

	CachedContentEncoding = []string{
		"none",
		"gzip",
		"br",
		"zstd",
	}
)

type Store struct {
	loc    string
	ttl    int
	logger *zap.Logger
	// memCach0 atomic.Value // *xsync.MapOf[string, *MemCacheItem]

	memMaxSize  int
	memMaxCount int
	memCache    atomic.Value // *LRUCache[string, *MemCacheItem]
}

type MemCacheItem struct {
	*CacheMeta
	value []byte
}

const (
	CACHE_DIR = "sidekick-cache"
)

func NewStore(loc string, ttl int, memMaxSize int, memMaxCount int, logger *zap.Logger) *Store {
	os.MkdirAll(loc+"/"+CACHE_DIR, 0o755)
	// memCache := xsync.NewMapOf[*MemCacheItem]()
	d := &Store{
		loc:    loc,
		ttl:    ttl,
		logger: logger,

		memMaxSize:  memMaxSize,
		memMaxCount: memMaxCount,
	}
	memCache := NewLRUCache[string, *MemCacheItem](memMaxCount, memMaxSize)
	d.memCache.Store(memCache)

	// Load cache from disk
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

// func (d *Store) getMemCache() *xsync.MapOf[string, *MemCacheItem] {
// 	memCache, ok := d.memCache.Load().(*xsync.MapOf[string, *MemCacheItem])
// 	if !ok {
// 		return nil
// 	}
// 	return memCache
// }

func (d *Store) getMemCache() *LRUCache[string, *MemCacheItem] {
	memCache, ok := d.memCache.Load().(*LRUCache[string, *MemCacheItem])
	if !ok {
		return nil
	}
	return memCache
}

func (d *Store) Get(key string, ce string) ([]byte, *CacheMeta, error) {
	key = strings.ReplaceAll(key, "/", "+")
	d.logger.Debug("Getting key from cache", zap.String("key", key), zap.String("ce", ce))

	memCache := d.getMemCache()

	// load from memory or try load from disk
	var retErr error
	var cacheItem *MemCacheItem
	isDisk := false
	cacheKey := key + "::" + ce
	for cacheItem == nil && retErr == nil {
		// not sure why compute function may get called more than once...?
		cacheItem, _ = memCache.LoadOrCompute(cacheKey, func() (*MemCacheItem, int, bool) {
			// test for disable load from disk
			// retErr = ErrCacheNotFound
			// return nil, 0, false

			cacheMeta := &CacheMeta{}
			err := cacheMeta.LoadFromFile(path.Join(d.loc, CACHE_DIR, key, ".meta"))
			if err != nil {
				retErr = err
				return nil, 0, false
			}
			value, err := os.ReadFile(path.Join(d.loc, CACHE_DIR, key, "."+ce))
			if err != nil {
				retErr = err
				return nil, 0, false
			}

			isDisk = true
			return &MemCacheItem{
				CacheMeta: cacheMeta,
				value:     value,
			}, len(value), true // TODO: add header size
		})
		if cacheItem == nil {
			memCache.Delete(cacheKey)
		}
	}
	if retErr != nil {
		d.logger.Debug("Error pulled key from disk", zap.String("key", key), zap.String("ce", ce), zap.Error(retErr))
		return nil, nil, ErrCacheNotFound
	}

	if isDisk {
		d.logger.Debug("Pulled key from disk", zap.String("key", key), zap.String("ce", ce))
	} else {
		d.logger.Debug("Pulled key from memory", zap.String("key", key), zap.String("ce", ce))
	}

	if time.Now().Unix()-cacheItem.Timestamp > int64(d.ttl) {
		d.logger.Debug("Cache expired", zap.String("key", key))
		// TODO: fix racing when purge running and setting new value with same key
		go d.Purge(key)
		return nil, nil, ErrCacheExpired
	}

	d.logger.Debug("Cache hit", zap.String("key", key), zap.String("ce", ce))
	return cacheItem.value, cacheItem.CacheMeta, nil
}

func (d *Store) Set(reqPath string, cacheKey string, meta *CacheMeta, value []byte) error {
	key := d.buildCacheKey(reqPath, cacheKey)
	d.logger.Debug("Cache Key", zap.String("Key", key), zap.String("ce", meta.contentEncoding))

	key = strings.ReplaceAll(key, "/", "+")
	ce := meta.contentEncoding
	memCache := d.getMemCache()
	// _, existed := memCache.LoadAndStore(key+"::"+ce, &MemCacheItem{
	// 	CacheMeta: meta,
	// 	value:     value,
	// })
	existed := memCache.Put(key+"::"+ce, &MemCacheItem{
		CacheMeta: meta,
		value:     value,
	}, len(value)) // TODO: add header size

	d.logger.Debug("-----------------------------------")
	d.logger.Debug("Setting key in cache", zap.String("key", key), zap.String("ce", meta.contentEncoding), zap.Bool("replace", existed))

	// create page directory
	basePath := path.Join(d.loc, CACHE_DIR, key)
	os.MkdirAll(basePath, 0o755)
	err := os.WriteFile(path.Join(basePath, "."+ce), value, 0o644)
	if err != nil {
		d.logger.Error("Error writing data to cache", zap.Error(err))
	}
	err = meta.WriteToFile(path.Join(basePath, ".meta"))
	if err != nil {
		d.logger.Error("Error writing meta to cache", zap.Error(err))
	}
	return nil
}

func (d *Store) Purge(key string) {
	key = strings.ReplaceAll(key, "/", "+")
	d.logger.Debug("Removing key from cache", zap.String("key", key))

	memCache := d.getMemCache()
	rmKeys := make([]string, 0, 4)
	memCache.Range(func(k string, v *MemCacheItem) bool {
		if strings.HasPrefix(k, key) {
			rmKeys = append(rmKeys, k)
		}
		return true
	})
	for _, k := range rmKeys {
		d.logger.Debug("Removing key from mem cache", zap.String("key", k))
		memCache.Delete(k)
	}

	basePath := path.Join(d.loc, CACHE_DIR)
	files, err := os.ReadDir(basePath)
	if err != nil {
		d.logger.Error("Error Removing key from disk cache", zap.Error(err))
		return
	}
	for _, f := range files {
		name := f.Name()
		if !strings.HasPrefix(name, key) {
			continue
		}
		fp := path.Join(basePath, name)
		err := os.RemoveAll(fp)
		if err != nil {
			d.logger.Error("Error Removing key from disk cache", zap.String("fp", fp), zap.Error(err))
		}
		// for _, name := range CachedContentEncoding {
		// 	err := os.Remove(path.Join(fp, "."+name))
		// 	if err != nil {
		// 		d.logger.Error("Error Removing key from disk cache", zap.String("fp", fp), zap.Error(err))
		// 	}
		// }
	}
}

func (d *Store) Flush() error {
	d.memCache.Store(NewLRUCache[string, *MemCacheItem](d.memMaxCount, d.memMaxSize))
	// return nil
	basePath := path.Join(d.loc, CACHE_DIR)
	files, err := os.ReadDir(basePath)
	if err != nil {
		d.logger.Error("Error flushing cache", zap.Error(err))
		return err
	}
	for _, f := range files {
		fp := path.Join(basePath, f.Name())
		err = os.RemoveAll(fp)
		if err != nil {
			d.logger.Error("Error flushing cache", zap.String("fp", fp), zap.Error(err))
		}
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

	basePath := path.Join(d.loc, CACHE_DIR)
	files, err := os.ReadDir(basePath)
	list["disk"] = make([]string, 0)

	if err == nil {
		for _, file := range files {
			if !file.IsDir() {
				continue
			}
			dirName := file.Name()
			fp := path.Join(basePath, dirName)
			for _, name := range CachedContentEncoding {
				ckPath := path.Join(fp, "."+name)
				_, err := os.Stat(ckPath)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				list["disk"] = append(list["disk"], dirName+"::"+name)
			}
		}
	}

	list["debug"] = []string{
		fmt.Sprintf("max_size=%v", d.memMaxSize),
		fmt.Sprintf("max_count=%v", d.memMaxCount),
		fmt.Sprintf("size=%v", memCache.Cost()),
		fmt.Sprintf("coun=%v", memCache.Size()),
	}

	return list
}

func (d *Store) buildCacheKey(reqPath string, cacheKey string) string {
	// cacheKey := contentEncoding + "::" + reqPath
	return fmt.Sprintf("%v::%v", reqPath, cacheKey)
}
