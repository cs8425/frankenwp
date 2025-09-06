package cache

import (
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

func NewCustomWriter(rw http.ResponseWriter, r *http.Request, db *Store, logger *zap.Logger, codes []string, cacheHeaderName string) *CustomWriter {
	nw := CustomWriter{
		ResponseWriter: rw,
		Request:        r,
		Store:          db,
		Logger:         logger,

		// keep original request info
		// origHeader: r.Header.Clone(),
		origUrl: *r.URL,

		cacheResponseCodes: codes,
		cacheHeaderName:    cacheHeaderName,
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

	mx              sync.Mutex
	contentEncoding string
	header          [][]string
}

func (r *CustomWriter) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// set cache on response end
func (r *CustomWriter) Close() error {
	r.mx.Lock()
	ct := r.contentEncoding
	r.mx.Unlock()
	r.Store.Set(r.origUrl.Path, ct, "", int(atomic.LoadInt32(&r.status)), r.buf)
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

	// TODO: more check response for bypass
	// if bypass {
	// 	bypass = strings.HasSuffix(r.origUrl.Path, ".php")
	// }

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

	cacheState := "BYPASS"
	if bypass {
		hdr.Set(r.cacheHeaderName, cacheState)
		r.ResponseWriter.WriteHeader(status)
		return
	}

	atomic.StoreInt32(&r.needCache, 1)
	cacheState = "MISS"

	// save response data
	// content encoding
	ct := hdr.Get("Content-Encoding")
	if ct == "" {
		ct = "none"
	}
	r.mx.Lock()
	r.contentEncoding = ct
	r.mx.Unlock()

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

	if atomic.LoadInt32(&r.needCache) == 1 {
		// assume Write() not called concurrently
		r.buf = append(r.buf, b...)
	}

	return r.ResponseWriter.Write(b)
}
