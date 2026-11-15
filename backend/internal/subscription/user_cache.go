package subscription

import (
	"container/list"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"gowave/internal/database"
)

var userCachedTables = []string{"users", "internal_squad_members", "user_traffic"}

var (
	userCachedTableSet = func() map[string]bool {
		set := make(map[string]bool, len(userCachedTables))
		for _, table := range userCachedTables {
			set[table] = true
		}
		return set
	}()
	userCachedTablePattern = regexp.MustCompile(`(?i)\b(?:` + strings.Join(userCachedTables, "|") + `)\b`)
)

func touchesUserTables(statement *gorm.Statement) bool {
	if statement == nil {
		return false
	}
	if statement.Table != "" && userCachedTableSet[statement.Table] {
		return true
	}
	if statement.Schema != nil && userCachedTableSet[statement.Schema.Table] {
		return true
	}
	return userCachedTablePattern.MatchString(statement.SQL.String())
}

// CachedUserData stores the user record and their internal squads.
type CachedUserData struct {
	User           *database.User
	InternalSquads []string
	ExpiresAt      time.Time
}

type userCacheItem struct {
	shortUUID string
	data      *CachedUserData
	element   *list.Element
}

// UserLRUCache is a thread-safe, bounded in-memory cache for user records
// and their squad memberships, automatically invalidated on user writes.
type UserLRUCache struct {
	mu         sync.Mutex
	capacity   int
	ttl        time.Duration
	items      map[string]*userCacheItem
	evict      *list.List
	sf         singleflight.Group
	generation uint64
}

func newUserLRUCache(db *gorm.DB, capacity int, ttl time.Duration) *UserLRUCache {
	if capacity <= 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = 48 * time.Hour
	}
	c := &UserLRUCache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*userCacheItem, capacity),
		evict:    list.New(),
	}
	if db != nil {
		c.registerInvalidator(db)
	}
	return c
}

func (c *UserLRUCache) registerInvalidator(db *gorm.DB) {
	notice := func(tx *gorm.DB) {
		if tx.Error == nil && touchesUserTables(tx.Statement) {
			c.Purge()
		}
	}
	callbacks := db.Callback()
	_ = callbacks.Create().After("gorm:commit_or_rollback_transaction").Register("subscription:user_cache_create", notice)
	_ = callbacks.Update().After("gorm:commit_or_rollback_transaction").Register("subscription:user_cache_update", notice)
	_ = callbacks.Delete().After("gorm:commit_or_rollback_transaction").Register("subscription:user_cache_delete", notice)
	_ = callbacks.Raw().After("gorm:raw").Register("subscription:user_cache_raw", notice)
}

func (c *UserLRUCache) Get(shortUUID string) (*CachedUserData, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[shortUUID]
	if !ok {
		return nil, false
	}
	if time.Now().After(item.data.ExpiresAt) {
		c.removeItem(item)
		return nil, false
	}
	c.evict.MoveToFront(item.element)
	return item.data, true
}

func (c *UserLRUCache) Generation() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}

// Set only publishes a load if no invalidation occurred since it started.
func (c *UserLRUCache) Set(shortUUID string, data *CachedUserData, generation uint64) {
	if c == nil || c.capacity <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if generation != c.generation {
		return
	}

	data.ExpiresAt = time.Now().Add(c.ttl)

	if item, ok := c.items[shortUUID]; ok {
		item.data = data
		c.evict.MoveToFront(item.element)
		return
	}

	if c.evict.Len() >= c.capacity {
		c.evictOldest()
	}

	item := &userCacheItem{
		shortUUID: shortUUID,
		data:      data,
	}
	item.element = c.evict.PushFront(item)
	c.items[shortUUID] = item
}

func (c *UserLRUCache) Delete(shortUUID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	if item, ok := c.items[shortUUID]; ok {
		c.removeItem(item)
	}
}

func (c *UserLRUCache) Purge() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.items = make(map[string]*userCacheItem, c.capacity)
	c.evict.Init()
}

func (c *UserLRUCache) removeItem(item *userCacheItem) {
	c.evict.Remove(item.element)
	delete(c.items, item.shortUUID)
}

func (c *UserLRUCache) evictOldest() {
	elem := c.evict.Back()
	if elem != nil {
		item := elem.Value.(*userCacheItem)
		c.removeItem(item)
	}
}

func cloneUser(u *database.User) *database.User {
	if u == nil {
		return nil
	}
	cp := *u
	if u.Traffic != nil {
		tr := *u.Traffic
		cp.Traffic = &tr
	}
	return &cp
}

func cloneSquads(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}
