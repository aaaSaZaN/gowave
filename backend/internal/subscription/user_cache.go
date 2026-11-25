package subscription

import (
	"container/list"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"gowave/internal/database"
)

var userCachedTables = []string{"users", "internal_squad_members", "user_traffic", "internal_squads", "external_squads"}

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

type userCacheShard struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*userCacheItem
	byID     map[uint64]*userCacheItem
	evict    *list.List
}

func newShard(capacity int) *userCacheShard {
	return &userCacheShard{
		capacity: capacity,
		items:    make(map[string]*userCacheItem, capacity),
		byID:     make(map[uint64]*userCacheItem, capacity),
		evict:    list.New(),
	}
}

func (s *userCacheShard) removeItem(item *userCacheItem) {
	s.evict.Remove(item.element)
	delete(s.items, item.shortUUID)
	delete(s.byID, item.data.User.ID)
}

func (s *userCacheShard) evictOldest() {
	elem := s.evict.Back()
	if elem != nil {
		item := elem.Value.(*userCacheItem)
		s.removeItem(item)
	}
}

func fnv32(key string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

// UserLRUCache is a thread-safe, bounded in-memory cache for user records
// and their squad memberships, automatically invalidated on user writes.
type UserLRUCache struct {
	shards     []*userCacheShard
	mask       uint32
	capacity   int
	ttl        time.Duration
	now        func() time.Time
	sf         singleflight.Group
	generation atomic.Uint64
}

func (c *UserLRUCache) shard(key string) *userCacheShard {
	if c.mask == 0 {
		return c.shards[0]
	}
	return c.shards[fnv32(key)&c.mask]
}

func newUserLRUCache(capacity int, ttl time.Duration) *UserLRUCache {
	if capacity <= 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = CacheTTL
	}
	numShards := 16
	if capacity < 16 {
		numShards = 1
	}
	shardCap := capacity / numShards
	if shardCap < 1 {
		shardCap = 1
	}
	c := &UserLRUCache{
		capacity: capacity,
		ttl:      ttl,
		now:      time.Now,
		shards:   make([]*userCacheShard, numShards),
		mask:     uint32(numShards - 1),
	}
	for i := 0; i < numShards; i++ {
		c.shards[i] = newShard(shardCap)
	}
	return c
}

func (c *UserLRUCache) Get(shortUUID string) (*CachedUserData, bool) {
	if c == nil {
		return nil, false
	}
	s := c.shard(shortUUID)
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[shortUUID]
	if !ok {
		return nil, false
	}
	if !c.now().Before(item.data.ExpiresAt) {
		s.removeItem(item)
		return nil, false
	}
	s.evict.MoveToFront(item.element)
	return item.data, true
}

func (c *UserLRUCache) Generation() uint64 {
	return c.generation.Load()
}

// Set only publishes a load if no invalidation occurred since it started.
func (c *UserLRUCache) Set(shortUUID string, data *CachedUserData, generation uint64) {
	if c == nil || c.capacity <= 0 {
		return
	}
	if generation != c.generation.Load() {
		return
	}
	s := c.shard(shortUUID)
	s.mu.Lock()
	defer s.mu.Unlock()

	if generation != c.generation.Load() {
		return
	}

	now := c.now()
	if data.ExpiresAt.IsZero() {
		data.ExpiresAt = now.Add(c.ttl)
	}
	if !now.Before(data.ExpiresAt) {
		return
	}
	if item, ok := s.items[shortUUID]; ok && item.data.ExpiresAt.After(data.ExpiresAt) {
		return
	}

	if item, ok := s.byID[data.User.ID]; ok && item.shortUUID != shortUUID {
		s.removeItem(item)
	}
	if item, ok := s.items[shortUUID]; ok {
		s.removeItem(item)
	}

	if s.evict.Len() >= s.capacity {
		s.evictOldest()
	}

	item := &userCacheItem{
		shortUUID: shortUUID,
		data:      data,
	}
	item.element = s.evict.PushFront(item)
	s.items[shortUUID] = item
	s.byID[data.User.ID] = item
}

func (c *UserLRUCache) invalidateUsers(ids []uint64) {
	// A cold load may not have reached the ID index yet. Advancing the
	// generation prevents it from publishing a pre-commit traffic value.
	c.generation.Add(1)
	for _, s := range c.shards {
		s.mu.Lock()
		for _, id := range ids {
			if item, ok := s.byID[id]; ok {
				s.removeItem(item)
			}
		}
		s.mu.Unlock()
	}
}

func (c *UserLRUCache) Purge() {
	if c == nil {
		return
	}
	c.generation.Add(1)
	for _, s := range c.shards {
		s.mu.Lock()
		clear(s.items)
		clear(s.byID)
		s.evict.Init()
		s.mu.Unlock()
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
