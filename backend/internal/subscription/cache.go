package subscription

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// The public subscription path reads the same settings, templates, external
// squads and host graph on every request. They are cached in memory and
// dropped whenever this process writes one of the tables below. Tests may
// change the durations, like subtpl.Now.
var (
	// CacheTTL bounds how long a snapshot is trusted without a local write,
	// so changes from another process (CLI import, a second instance) still
	// show up.
	CacheTTL = 5 * time.Second
	// CacheSettle disables caching for a while after a local write: a
	// callback runs before an enclosing transaction commits, and a load made
	// in between would still see the old rows.
	CacheSettle = 2 * time.Second
)

// cachedTables are every table the cached values are read from, plus the
// parents whose deletes cascade into them.
var cachedTables = []string{
	"subscription_settings", "subscription_templates", "external_squads", "external_squads_templates",
	"hosts", "config_profiles", "config_profile_inbounds", "internal_squads", "internal_squad_inbounds", "internal_squad_host_links",
}

var (
	cachedTableSet = func() map[string]bool {
		set := make(map[string]bool, len(cachedTables))
		for _, table := range cachedTables {
			set[table] = true
		}
		return set
	}()
	// cachedTablePattern finds the tables in raw SQL; \b keeps
	// internal_squad_members or config_profile_inbounds_to_nodes from matching.
	cachedTablePattern = regexp.MustCompile(`(?i)\b(?:` + strings.Join(cachedTables, "|") + `)\b`)
)

// changeTracker counts writes to cachedTables through one *gorm.DB.
type changeTracker struct {
	generation atomic.Uint64
	changedAt  atomic.Int64 // UnixNano of the last write
}

func (t *changeTracker) bump(now time.Time) {
	// changedAt first: a reader that misses the new generation still sees
	// the settle window.
	t.changedAt.Store(now.UnixNano())
	t.generation.Add(1)
}

func (t *changeTracker) settling(now time.Time) bool {
	return now.UnixNano()-t.changedAt.Load() < int64(CacheSettle)
}

// trackers holds one tracker per callback registry; sessions and
// transactions of a database share it.
var trackers sync.Map

func trackerFor(db *gorm.DB) *changeTracker {
	key := db.Callback()
	if tracker, ok := trackers.Load(key); ok {
		return tracker.(*changeTracker)
	}
	tracker := &changeTracker{}
	if existing, loaded := trackers.LoadOrStore(key, tracker); loaded {
		return existing.(*changeTracker)
	}
	notice := func(tx *gorm.DB) {
		if touchesCachedTables(tx.Statement) {
			tracker.bump(time.Now())
		}
	}
	callbacks := db.Callback()
	_ = callbacks.Create().After("gorm:create").Register("subscription:cache_create", notice)
	_ = callbacks.Update().After("gorm:update").Register("subscription:cache_update", notice)
	_ = callbacks.Delete().After("gorm:delete").Register("subscription:cache_delete", notice)
	_ = callbacks.Raw().After("gorm:raw").Register("subscription:cache_raw", notice)
	return tracker
}

func touchesCachedTables(statement *gorm.Statement) bool {
	if statement.Table != "" {
		if cachedTableSet[statement.Table] {
			return true
		}
	} else if statement.Schema != nil && cachedTableSet[statement.Schema.Table] {
		return true
	}
	// Raw Exec has no table; a model statement on another table may still
	// name one in a subquery.
	return cachedTablePattern.MatchString(statement.SQL.String())
}

// lazy computes a value once; failed loads are retried.
type lazy[T any] struct {
	mu    sync.Mutex
	ready atomic.Pointer[T]
}

func (l *lazy[T]) get(load func() (T, error)) (T, error) {
	if value := l.ready.Load(); value != nil {
		return *value, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if value := l.ready.Load(); value != nil {
		return *value, nil
	}
	value, err := load()
	if err == nil {
		l.ready.Store(&value)
	}
	return value, err
}

func keyed[T any](values *sync.Map, key any) *lazy[T] {
	if value, ok := values.Load(key); ok {
		return value.(*lazy[T])
	}
	value, _ := values.LoadOrStore(key, &lazy[T]{})
	return value.(*lazy[T])
}

// snapshot is the cached state of one generation.
type snapshot struct {
	generation uint64
	createdAt  time.Time

	settings lazy[*subSettings]
	hosts    lazy[*hostIndex]
	// templates: templateKey -> templateResult; squads: uuid -> squad row
	// (nil when missing); squadTemplates: templateKey -> name.
	templates      sync.Map
	squads         sync.Map
	squadTemplates sync.Map
}

type subCache struct {
	tracker *changeTracker
	now     func() time.Time
	current atomic.Pointer[snapshot]
}

func newSubCache(db *gorm.DB) *subCache {
	return &subCache{tracker: trackerFor(db), now: time.Now}
}

// snapshot returns the live snapshot, or nil while a recent write settles
// or without a cache; callers then read the database directly.
func (c *subCache) snapshot() *snapshot {
	if c == nil {
		return nil
	}
	now := c.now()
	generation := c.tracker.generation.Load()
	current := c.current.Load()
	if current != nil && current.generation == generation && now.Sub(current.createdAt) < CacheTTL {
		return current
	}
	if c.tracker.settling(now) {
		return nil
	}
	fresh := &snapshot{generation: generation, createdAt: now}
	if c.current.CompareAndSwap(current, fresh) {
		return fresh
	}
	// Another request replaced it first; share its loads.
	if latest := c.current.Load(); latest != nil && latest.generation == generation {
		return latest
	}
	return fresh
}
