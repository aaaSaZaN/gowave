package subscription

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"gowave/internal/database"
)

// The public subscription path reads the same settings, templates, external
// squads and host graph on every request. They are cached in memory and
// dropped after local writes. The TTL bounds visibility of changes from
// another process (CLI import or a second instance).
var CacheTTL = 5 * time.Second

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

// Register once at handler construction. Schedule invalidations after the SQL
// statement so they run on commit of the actual containing transaction.
func (h *Handler) registerCacheInvalidation() {
	notice := func(tx *gorm.DB) {
		if tx.Error != nil || tx.DryRun {
			return
		}
		if change := database.GetUserTrafficChange(tx); change != nil {
			if h.userCache != nil {
				database.AfterCommit(tx, change, func() { h.userCache.invalidateUsers(change.IDs) })
			}
			return
		}
		if touchesCachedTables(tx.Statement) {
			database.AfterCommit(tx, h.cache, func() { h.cache.generation.Add(1) })
		}
		if h.userCache != nil && touchesUserTables(tx.Statement) {
			database.AfterCommit(tx, h.userCache, h.userCache.Purge)
		}
	}
	callbacks := h.db.Callback()
	_ = callbacks.Create().After("gorm:create").Before("gorm:commit_or_rollback_transaction").Register("subscription:cache_create", notice)
	_ = callbacks.Update().After("gorm:update").Before("gorm:commit_or_rollback_transaction").Register("subscription:cache_update", notice)
	_ = callbacks.Delete().After("gorm:delete").Before("gorm:commit_or_rollback_transaction").Register("subscription:cache_delete", notice)
	_ = callbacks.Raw().After("gorm:raw").Register("subscription:cache_raw", notice)
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
	generation atomic.Uint64
	now        func() time.Time
	current    atomic.Pointer[snapshot]
}

// snapshot returns the live snapshot. A load already holding an older
// snapshot can finish there without publishing into the new generation.
func (c *subCache) snapshot() *snapshot {
	if c == nil {
		return nil
	}
	for {
		current := c.current.Load()
		generation := c.generation.Load()
		now := c.now()
		if current != nil && current.generation == generation && now.Sub(current.createdAt) < CacheTTL {
			return current
		}
		fresh := &snapshot{generation: generation, createdAt: now}
		if c.current.CompareAndSwap(current, fresh) {
			return fresh
		}
	}
}
