package cache

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"sync/atomic"

	"go.uber.org/zap"
)

func NewCustomWriter(rw http.ResponseWriter, r *http.Request, db *Store, logger *zap.Logger, c *Cache) *CustomWriter {
	nw := CustomWriter{
		ResponseWriter: rw,
		Request:        r,
		Store:          db,
		Logger:         logger,

		// keep original request info
		// origHeader: r.Header.Clone(),
		origUrl: *r.URL,

		cacheMaxSize:       c.MemoryItemCacheMaxSize,
		cacheResponseCodes: c.CacheResponseCodes,
		cacheHeaderName:    c.CacheHeaderName,
		status:             -1,
	}
	return &nw
}

var _ http.ResponseWriter = (*CustomWriter)(nil)

// CustomWriter handles the response and provide the way to cache the value
type CustomWriter struct {
	http.ResponseWriter
	*http.Request
	*Store
	*zap.Logger
	cacheResponseCodes []string
	cacheHeaderName    string
	cacheMaxSize       int

	// origHeader http.Header
	origUrl url.URL

	// -1 means header not send yet
	status int32

	// flag response data need to be cached
	needCache int32

	// currently cache in memory
	// assume response data not too large
	// TODO: buffer pool
	buf []byte
}

func (r *CustomWriter) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// set cache on response end
func (r *CustomWriter) Close() error {
	if atomic.LoadInt32(&r.needCache) == 1 {
		hdr := r.ResponseWriter.Header()
		meta := NewCacheMeta(int(atomic.LoadInt32(&r.status)), hdr)
		if meta == nil {
			return nil
		}
		r.Store.Set(r.origUrl.Path, "", meta, r.buf)
	}
	return nil
}

func (r *CustomWriter) Header() http.Header {
	return r.ResponseWriter.Header()
}

func (r *CustomWriter) WriteHeader(status int) {
	r.Logger.Debug("==========-SetHeader-==========")
	atomic.StoreInt32(&r.status, int32(status))

	r.Logger.Debug("Writing customwriter response", zap.String("path", r.origUrl.Path))
	bypass := true

	// check if the response code is in the cache response codes
	if bypass {
		statusStr := strconv.Itoa(status)
		for _, code := range r.cacheResponseCodes {
			r.Logger.Debug("Checking status code", zap.String("code", code), zap.String("status", statusStr))

			if code == statusStr {
				r.Logger.Debug("Caching because of status code", zap.String("code", code), zap.String("status", statusStr))
				bypass = false
				break
			}

			// code may be single digit because of wildcard usage (e.g. 2XX, 4XX, 5XX)
			if len(code) == 1 {
				if code == statusStr[0:1] {
					r.Logger.Debug("Caching because of wildcard", zap.String("code", code), zap.String("status", statusStr))
					bypass = false
					break
				}
			}
		}
	}

	// TODO: check if data if too large, then write to temporary file
	// TODO: more bypass rule by config
	hdr := r.Header()

	// check if response should not cached
	for h := range hdr {
		ok := slices.Contains(hdrResNotCacheList, h)
		if ok {
			bypass = true
			break
		}
	}

	cacheState := "BYPASS"
	if bypass {
		hdr.Set(r.cacheHeaderName, cacheState)
		r.ResponseWriter.WriteHeader(status)
		return
	}

	atomic.StoreInt32(&r.needCache, 1)
	cacheState = "MISS"

	// TODO: prevent multiple CustomWriter cache when concurrent request same page (same cacheKey)

	hdr.Set(r.cacheHeaderName, cacheState)
	r.ResponseWriter.WriteHeader(status)
}

// Write will write the response body
func (r *CustomWriter) Write(b []byte) (int, error) {
	// check header has been written or not
	if atomic.CompareAndSwapInt32(&r.status, -1, 200) {
		r.WriteHeader(200)
	}

	// save response data
	if atomic.LoadInt32(&r.needCache) == 1 {
		sz := len(r.buf) + len(b)
		if sz <= r.cacheMaxSize {
			// assume Write() not called concurrently
			r.buf = append(r.buf, b...)
		} else {
			// TODO: rewrite to temporary file on disk?
			// too large, skip cache in memory
			atomic.StoreInt32(&r.needCache, 0)
			r.buf = nil

			r.Logger.Debug("Bypass caching because of data size", zap.Int("sz", sz), zap.Int("limit", r.cacheMaxSize))
		}
	}

	return r.ResponseWriter.Write(b)
}
