package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ilog "github.com/shidaxi/iaws/internal/log"
)

const (
	// shared cache dir with AWS CLI, enabling credential sharing with aws / kubectl (EKS uses the same credential chain)
	credCacheDir = ".aws/cli/cache"
	credCacheTTL = 5 * time.Minute // refresh ahead of expiry
	iawsPrefix   = "iaws_"          // filename prefix for our files, to avoid conflicts with CLI hash filenames
)

type cachedCreds struct {
	AccessKeyID     string    `json:"AccessKeyId"`
	SecretAccessKey string    `json:"SecretAccessKey"`
	SessionToken    string    `json:"SessionToken"`
	Expiration      time.Time `json:"Expiration"`
}

// fileCacheProvider wraps a CredentialsProvider, preferring disk-cached assume-role credentials; on miss or expiry, calls the inner provider and writes back to cache.
type fileCacheProvider struct {
	inner   aws.CredentialsProvider
	profile string
	region  string
}

func newFileCacheProvider(inner aws.CredentialsProvider, profile, region string) aws.CredentialsProvider {
	if inner == nil {
		return inner
	}
	return &fileCacheProvider{inner: inner, profile: profile, region: region}
}

func (p *fileCacheProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	ilog.Debug("cred cache: retrieve for profile=%q region=%q", p.profile, p.region)
	creds, ok := p.loadCache()
	if ok && (creds.Expires.IsZero() || creds.Expires.After(time.Now().Add(credCacheTTL))) {
		ilog.Info("cred cache: hit, expires=%v", creds.Expires)
		return creds, nil
	}
	if ok {
		ilog.Info("cred cache: expired (expires=%v), refreshing", creds.Expires)
	} else {
		ilog.Info("cred cache: miss, calling inner provider")
	}
	creds, err := p.inner.Retrieve(ctx)
	if err != nil {
		ilog.Error("cred cache: inner provider failed: %v", err)
		return aws.Credentials{}, err
	}
	ilog.Info("cred cache: got credentials, expires=%v, saving to disk", creds.Expires)
	p.saveCache(creds)
	return creds, nil
}

func (p *fileCacheProvider) cacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, credCacheDir)
}

func (p *fileCacheProvider) cachePath() string {
	safe := strings.ReplaceAll(p.profile+"_"+p.region, "/", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	return filepath.Join(p.cacheDir(), iawsPrefix+safe+".json")
}

func (p *fileCacheProvider) loadCache() (aws.Credentials, bool) {
	// 1) first try our own cache (iaws_<profile>_<region>.json)
	path := p.cachePath()
	if creds, ok := p.loadCacheFile(path); ok {
		ilog.Debug("cred cache: loaded from own file %s", path)
		return creds, true
	}
	// 2) then try iaws_<profile>_.json (filename written when region is empty)
	if p.region != "" {
		pathNoRegion := filepath.Join(p.cacheDir(), iawsPrefix+strings.ReplaceAll(strings.ReplaceAll(p.profile, "/", "_"), ":", "_")+".json")
		if creds, ok := p.loadCacheFile(pathNoRegion); ok {
			ilog.Debug("cred cache: loaded from no-region file %s", pathNoRegion)
			return creds, true
		}
	}
	// 3) on miss, try other cache files in the same directory (e.g. written by AWS CLI); reuse any valid one
	dir := p.cacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		ilog.Debug("cred cache: cannot read dir %s: %v", dir, err)
		return aws.Credentials{}, false
	}
	now := time.Now()
	deadline := now.Add(credCacheTTL)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if strings.HasPrefix(e.Name(), iawsPrefix) {
			continue
		}
		fp := filepath.Join(dir, e.Name())
		creds, ok := p.loadCacheFile(fp)
		if !ok || creds.Expires.IsZero() || creds.Expires.Before(deadline) {
			continue
		}
		ilog.Debug("cred cache: reusing CLI cache file %s", fp)
		return creds, true
	}
	ilog.Debug("cred cache: no valid cache found in %s", dir)
	return aws.Credentials{}, false
}

func (p *fileCacheProvider) loadCacheFile(path string) (aws.Credentials, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return aws.Credentials{}, false
	}
	c, ok := parseCredsJSON(b)
	if !ok || c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return aws.Credentials{}, false
	}
	return aws.Credentials{
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		SessionToken:    c.SessionToken,
		CanExpire:       !c.Expiration.IsZero(),
		Expires:         c.Expiration,
	}, true
}

// parseCredsJSON parses multiple common JSON formats (iaws, credential_process, CLI cache, etc.)
func parseCredsJSON(b []byte) (c cachedCreds, ok bool) {
	// 1) direct format: AccessKeyId, SecretAccessKey, SessionToken, Expiration
	if json.Unmarshal(b, &c) == nil && c.AccessKeyID != "" {
		return c, true
	}
	// 2) top-level Version + fields, or nested Credentials
	var wrapper struct {
		Version        int    `json:"Version"`
		AccessKeyId    string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		SessionToken   string `json:"SessionToken"`
		Expiration     string `json:"Expiration"` // commonly ISO8601 string
		Credentials    *struct {
			AccessKeyId    string `json:"AccessKeyId"`
			SecretAccessKey string `json:"SecretAccessKey"`
			SessionToken   string `json:"SessionToken"`
			Expiration     string `json:"Expiration"`
		} `json:"Credentials"`
	}
	if json.Unmarshal(b, &wrapper) != nil {
		return cachedCreds{}, false
	}
	if wrapper.Credentials != nil {
		c.AccessKeyID = wrapper.Credentials.AccessKeyId
		c.SecretAccessKey = wrapper.Credentials.SecretAccessKey
		c.SessionToken = wrapper.Credentials.SessionToken
		c.Expiration, _ = parseExpiration(wrapper.Credentials.Expiration)
	} else {
		c.AccessKeyID = wrapper.AccessKeyId
		c.SecretAccessKey = wrapper.SecretAccessKey
		c.SessionToken = wrapper.SessionToken
		c.Expiration, _ = parseExpiration(wrapper.Expiration)
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		// 3) try lowercase+underscore key names (some CLI/botocore formats)
		c, ok = parseCredsJSONMap(b)
		return c, ok
	}
	return c, true
}

// parseCredsJSONMap extracts credentials from arbitrary JSON by common key names (including nested)
func parseCredsJSONMap(b []byte) (c cachedCreds, ok bool) {
	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return cachedCreds{}, false
	}
	getStr := func(key ...string) string {
		for _, k := range key {
			if v, exists := m[k]; exists {
				if s, _ := v.(string); s != "" {
					return s
				}
			}
		}
		if creds, _ := m["Credentials"].(map[string]interface{}); creds != nil {
			for _, k := range key {
				if v, exists := creds[k]; exists {
					if s, _ := v.(string); s != "" {
						return s
					}
				}
			}
		}
		return ""
	}
	c.AccessKeyID = getStr("AccessKeyId", "access_key_id", "AccessKeyID")
	c.SecretAccessKey = getStr("SecretAccessKey", "secret_access_key")
	c.SessionToken = getStr("SessionToken", "session_token")
	getExp := func(key ...string) string {
		for _, k := range key {
			if v, exists := m[k]; exists {
				if s, _ := v.(string); s != "" {
					return s
				}
			}
		}
		if creds, _ := m["Credentials"].(map[string]interface{}); creds != nil {
			for _, k := range key {
				if v, exists := creds[k]; exists {
					if s, _ := v.(string); s != "" {
						return s
					}
				}
			}
		}
		return ""
	}
	if expStr := getExp("Expiration", "expiration"); expStr != "" {
		c.Expiration, _ = parseExpiration(expStr)
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return cachedCreds{}, false
	}
	return c, true
}

func parseExpiration(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, true
	}
	t, err = time.Parse("2006-01-02T15:04:05Z07:00", s)
	if err == nil {
		return t, true
	}
	t, err = time.Parse("2006-01-02T15:04:05.999999999Z07:00", s)
	if err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (p *fileCacheProvider) saveCache(creds aws.Credentials) {
	if creds.SessionToken == "" {
		return
	}
	dir := filepath.Dir(p.cachePath())
	_ = os.MkdirAll(dir, 0700)
	c := cachedCreds{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Expiration:      creds.Expires,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(p.cachePath(), b, 0600)
}
