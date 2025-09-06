package cache

import (
	"encoding/json"
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
	CacheResponseCodes []string
	TTL                int
	Store              *Store

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

	c.Store = NewStore(c.Loc, c.TTL, c.logger)

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
func (c Cache) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	bypass := false
	encoding := ""

	c.logger.Debug("HTTP Version", zap.String("Version", r.Proto))

	for _, prefix := range c.BypassPathPrefixes {
		if strings.HasPrefix(r.URL.Path, prefix) && prefix != "" {
			c.logger.Debug("wp cache - bypass prefix", zap.String("prefix", prefix))
			bypass = true
			break
		}
	}

	// bypass by regex
	// default: ".*(\\.[^.]+)$", bypass all media, images, css, js, etc
	if c.pathRx != nil {
		bypass = c.pathRx.MatchString(r.URL.Path)
		c.logger.Debug("wp cache - bypass regex", zap.String("regex", c.BypassPathRegex))
	}

	if c.BypassHome && r.URL.Path == "/" {
		bypass = true
	}

	if bypass {
		return next.ServeHTTP(w, r)
	}

	db := c.Store
	if strings.HasPrefix(r.URL.Path, c.PurgePath) {
		key := r.Header.Get(c.PurgeKeyHeader)
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

	// bypass if is logged in. We don't want to cache admin bars
	cookies := r.Header.Get("Cookie")
	if strings.Contains(cookies, "wordpress_logged_in") {
		return next.ServeHTTP(w, r)
	}

	requestHeader := r.Header
	requestEncoding := requestHeader["Accept-Encoding"]

	for _, re := range requestEncoding {
		if strings.Contains(re, "br") {
			encoding = "br"
			break
		} else if strings.Contains(re, "gzip") {
			encoding = "gzip"
		}
	}

	if encoding == "" {
		encoding = "none"
	}

	// TODO: custom cacheKey by query, header ...
	cacheKey := ""
	cacheKey = c.Store.buildCacheKey(r.URL.Path, encoding, cacheKey)
	cacheData, stateCode, err := db.Get(cacheKey)

	if err != nil {
		c.logger.Debug("wp cache - error - "+cacheKey, zap.Error(err))
	}

	if err == nil {
		// TODO: set original status code
		w.Header().Set(c.CacheHeaderName, "HIT")
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Header().Set("Server", "Caddy")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", encoding)
		w.WriteHeader(stateCode)
		w.Write(cacheData)

		return nil
	}

	nw := NewCustomWriter(w, r, db, c.logger, c.CacheResponseCodes, c.CacheHeaderName)
	defer nw.Close()
	return next.ServeHTTP(nw, r)
}
