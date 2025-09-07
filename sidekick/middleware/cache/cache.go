package cache

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

type Cache struct {
	logger             *zap.Logger
	Loc                string
	PurgePath          string
	PurgeKeyHeader     string
	PurgeKey           string
	CacheHeaderName    string
	BypassPathPrefixes []string
	BypassPathRegex    string
	BypassHome         bool
	BypassDebugQuery   string
	CacheResponseCodes []string
	TTL                int
	Store              *Store

	MemoryItemMaxSize   int
	MemoryCacheMaxSize  int
	MemoryCacheMaxCount int

	pathRx *regexp.Regexp
}

func init() {
	caddy.RegisterModule(Cache{})
	httpcaddyfile.RegisterHandlerDirective("wp_cache", parseCaddyfileHandler)
}

func parseCaddyfileHandler(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler,
	error) {
	c := new(Cache)
	if err := c.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Cache) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		var value string

		key := d.Val()

		if !d.Args(&value) {
			continue
		}

		switch key {
		case "loc":
			c.Loc = value

		case "bypass_path_prefixes":
			c.BypassPathPrefixes = strings.Split(strings.TrimSpace(value), ",")

		case "bypass_path_regex":
			value = strings.TrimSpace(value)
			if len(value) != 0 {
				_, err := regexp.Compile(value)
				if err != nil {
					return err
				}
			} else {
				// bypass all media, images, css, js, etc
				value = ".*(\\.[^.]+)$"
			}
			c.BypassPathRegex = value

		case "bypass_home":
			if strings.ToLower(value) == "true" {
				c.BypassHome = true
			}

		case "bypass_debug_query":
			c.BypassDebugQuery = strings.TrimSpace(value)

		case "cache_response_codes":
			codes := strings.Split(strings.TrimSpace(value), ",")
			c.CacheResponseCodes = make([]string, len(codes))

			for i, code := range codes {
				code = strings.TrimSpace(code)
				if strings.Contains(code, "XX") {
					code = string(code[0])
				}
				c.CacheResponseCodes[i] = code
			}

		case "ttl":
			ttl, err := strconv.Atoi(value)
			if err != nil {
				c.logger.Error("Invalid TTL value", zap.Error(err))
				continue
			}
			c.TTL = ttl

		case "purge_path":
			c.PurgePath = value

		case "purge_key":
			c.PurgeKey = strings.TrimSpace(value)

		case "purge_key_header":
			c.PurgeKeyHeader = value

		case "cache_header_name":
			c.CacheHeaderName = value

		case "memory_item_max_size":
			if n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				c.MemoryItemMaxSize = int(n)
			}

		case "memory_max_size":
			if n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				c.MemoryCacheMaxSize = int(n)
			}
		case "memory_max_count":
			if n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				c.MemoryCacheMaxCount = int(n)
			}
		}
	}

	return nil
}

func (c *Cache) Provision(ctx caddy.Context) error {
	c.logger = ctx.Logger(c)

	if c.Loc == "" {
		c.Loc = os.Getenv("CACHE_LOC")
	}

	if c.CacheResponseCodes == nil {
		codes := strings.Split(os.Getenv("CACHE_RESPONSE_CODES"), ",")
		c.CacheResponseCodes = make([]string, len(codes))

		for i, code := range codes {
			code = strings.TrimSpace(code)
			if strings.Contains(code, "XX") {
				code = string(code[0])
			}
			c.CacheResponseCodes[i] = code
		}
	}

	if c.BypassPathPrefixes == nil {
		c.BypassPathPrefixes = strings.Split(strings.TrimSpace(os.Getenv("BYPASS_PATH_PREFIX")), ",")
	}

	if c.BypassPathRegex == "" {
		// default bypass all media, images, css, js, etc
		c.BypassPathRegex = ".*(\\.[^.]+)$"
	}
	if c.BypassPathRegex != "" {
		rx, err := regexp.Compile(c.BypassPathRegex)
		if err != nil {
			return err
		}
		c.pathRx = rx
	}

	if !c.BypassHome {
		if strings.ToLower(os.Getenv("BYPASS_HOME")) == "true" {
			c.BypassHome = true
		}
	}

	if c.BypassDebugQuery == "" {
		c.BypassDebugQuery = os.Getenv("BYPASS_DEBUG_QUERY")
		if c.BypassDebugQuery == "" {
			c.BypassDebugQuery = "WPEverywhere-NOCACHE"
		}
	}

	if c.TTL == 0 {
		ttl, err := strconv.Atoi(os.Getenv("TTL"))
		if err != nil {
			c.logger.Error("Invalid TTL value", zap.Error(err))
		}
		c.TTL = ttl
	}

	if c.PurgePath == "" {
		c.PurgePath = os.Getenv("PURGE_PATH")

		if c.PurgePath == "" {
			c.PurgePath = "/__wp_cache/purge"
		}
	}

	if c.PurgeKey == "" {
		c.PurgeKey = os.Getenv("PURGE_KEY")
	}

	if c.PurgeKeyHeader == "" {
		c.PurgeKeyHeader = os.Getenv("PURGE_KEY_HEADER")
		if c.PurgeKeyHeader == "" {
			c.PurgeKeyHeader = "X-WPSidekick-Purge-Key"
		}
	}

	if c.CacheHeaderName == "" {
		c.CacheHeaderName = os.Getenv("CACHE_HEADER_NAME")
		if c.CacheHeaderName == "" {
			c.CacheHeaderName = "X-WPEverywhere-Cache"
		}
	}

	// TODO: let 0 == disable memory but cache to disk?
	if c.MemoryItemMaxSize == 0 {
		c.MemoryItemMaxSize = 4 * 1024 * 1024 // 4MB
	}
	if c.MemoryItemMaxSize < 0 { // < 0 == unlimited
		c.MemoryItemMaxSize = math.MaxInt
	}

	// TODO: let < 0 disable memory but cache to disk?
	if c.MemoryCacheMaxSize == 0 {
		c.MemoryCacheMaxSize = 128 * 1024 * 1024 // 128MB as default should be enough?
	}

	// TODO: let < 0 disable memory but cache to disk?
	if c.MemoryCacheMaxCount == 0 {
		c.MemoryCacheMaxCount = 32 * 1024 // 32K item as default should be enough?
	}

	c.Store = NewStore(c.Loc, c.TTL, c.MemoryCacheMaxSize, c.MemoryCacheMaxCount, c.logger)

	return nil
}

func (Cache) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.wp_cache",
		New: func() caddy.Module {
			return new(Cache)
		},
	}
}

// ServeHTTP implements the caddy.Handler interface.
func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	bypass := false
	c.logger.Debug("HTTP Version", zap.String("Version", r.Proto))

	reqHdr := r.Header
	db := c.Store
	if strings.HasPrefix(r.URL.Path, c.PurgePath) {
		key := reqHdr.Get(c.PurgeKeyHeader)
		if key != c.PurgeKey {
			c.logger.Warn("wp cache - purge - invalid key", zap.String("path", r.URL.Path))
		} else {
			switch r.Method {
			case "GET":
				cacheList := db.List()
				json.NewEncoder(w).Encode(cacheList)
				return nil

			case "POST":
				pathToPurge := strings.Replace(r.URL.Path, c.PurgePath, "", 1)
				c.logger.Debug("wp cache - purge", zap.String("path", pathToPurge))

				// TODO: fix concurrent issue when flush/pruge running and new cache setting
				if len(pathToPurge) < 2 {
					go db.Flush()
				} else {
					go db.Purge(pathToPurge)
				}
				w.Write([]byte("OK"))
				return nil
			}
		}
	}

	// only GET Method can cache
	if r.Method != "GET" {
		return next.ServeHTTP(w, r)
	}

	if c.BypassDebugQuery != "" {
		bypass = r.URL.Query().Has(c.BypassDebugQuery)
	}

	if !bypass {
		for _, prefix := range c.BypassPathPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) && prefix != "" {
				c.logger.Debug("wp cache - bypass prefix", zap.String("prefix", prefix))
				bypass = true
				break
			}
		}
	}

	// bypass by regex
	// default: ".*(\\.[^.]+)$", bypass all media, images, css, js, etc
	if !bypass && c.pathRx != nil {
		bypass = c.pathRx.MatchString(r.URL.Path)
		if bypass {
			c.logger.Debug("wp cache - bypass regex", zap.String("regex", c.BypassPathRegex))
		}
	}

	if !bypass && c.BypassHome && r.URL.Path == "/" {
		bypass = true
	}

	// bypass if is logged in. We don't want to cache admin bars
	if !bypass {
		cookies := r.Cookies()
		for _, cookie := range cookies {
			if strings.HasPrefix(cookie.Name, "wordpress_logged_in") {
				bypass = true
				break
			}
		}
	}

	hdr := w.Header()
	if bypass {
		hdr.Set(c.CacheHeaderName, "BYPASS")
		return next.ServeHTTP(w, r)
	}

	// TODO: custom cacheKey by query, header ...
	cacheKey := ""
	cacheKey = c.Store.buildCacheKey(r.URL.Path, cacheKey)

	requestEncoding := strings.Split(strings.Join(reqHdr["Accept-Encoding"], ""), ",")
	if len(requestEncoding) == 1 && len(requestEncoding[0]) == 0 {
		requestEncoding = nil
	}
	requestEncoding = append(requestEncoding, "none")

	// TODO: if only have uncompressed data, we should try to cached a compressed version
	var cacheData []byte
	var cacheMeta *CacheMeta
	var err error
	ce := ""
	for _, re := range requestEncoding {
		ce = strings.TrimSpace(re)
		cacheData, cacheMeta, err = db.Get(cacheKey, ce)
		if err == nil {
			break
		}
	}
	if err == nil {
		// TODO: some limit prevent self-DoS
		go c.doCache(r, next)

		// TODO: implement 304 Not Modified reponse for
		// ETag (If-Match, If-None-Match)
		// Last-Modified (If-Modified-Since, If-Unmodified-Since)

		hdr.Set(c.CacheHeaderName, "HIT")
		hdr.Set("Vary", "Accept-Encoding")
		if ce != "none" {
			hdr.Set("Content-Encoding", ce)
		}
		// hdr.Set("Content-Type", "text/html; charset=UTF-8")
		// hdr.Set("Server", "Caddy")
		for _, kv := range cacheMeta.Header {
			if len(kv) != 2 {
				continue
			}
			hdr.Set(kv[0], kv[1])
		}
		w.WriteHeader(cacheMeta.StateCode)
		w.Write(cacheData)

		return nil
	}
	c.logger.Debug("wp cache - error - "+cacheKey, zap.Error(err))

	nw := NewCustomWriter(w, r, db, c.logger, c)
	defer nw.Close()
	return next.ServeHTTP(nw, r)
}

func (c *Cache) doCache(r0 *http.Request, next caddyhttp.Handler) {
	r := r0.Clone(context.Background())
	repl := caddy.NewReplacer()
	r = caddyhttp.PrepareRequest(r, repl, nil, nil)
	c.logger.Debug("wp cache - preload - ", zap.String("path", r.URL.Path))
	db := c.Store
	w := &NopResponseWriter{}
	nw := NewCustomWriter(w, r, db, c.logger, c)
	defer nw.Close()
	next.ServeHTTP(nw, r)
}

// Interface guards
var (
	_ caddy.Provisioner           = (*Cache)(nil)
	_ caddyhttp.MiddlewareHandler = (*Cache)(nil)
	_ caddyfile.Unmarshaler       = (*Cache)(nil)
	// _ caddy.Validator             = (*Cache)(nil)

	_ http.ResponseWriter = (*NopResponseWriter)(nil)
)

type NopResponseWriter map[string][]string

func (nop *NopResponseWriter) WriteHeader(statusCode int) {}

func (nop *NopResponseWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

func (nop *NopResponseWriter) Header() http.Header {
	return http.Header(*nop)
}
