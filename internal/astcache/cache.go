// Package astcache keeps parsed MJML templates in memory for a fixed time, so
// that rendering the same template again skips the parse.
package astcache

import (
	"errors"
	"sync"
	"time"

	"github.com/Notifuse/safe-mjml/internal/parser"
)

// DefaultTTL is the time an entry is kept when New is given no positive TTL.
const DefaultTTL = 5 * time.Minute

// Cache maps a template to its parsed tree for a fixed time (TTL) from the
// parse; a hit does not extend it. Entries are never modified once stored, so
// renders may share them. Concurrent parses of the same template wait for the
// first one (single flight). A background goroutine removes the expired
// entries every TTL/2 until Close. A Cache is safe for concurrent use.
type Cache struct {
	ttl     time.Duration
	entries sync.Map // template string → *entry

	mu      sync.Mutex
	calls   map[string]*call // parses in progress
	closed  bool
	stop    chan struct{} // closed by the first Close, to stop the cleanup
	stopped chan struct{} // closed by the cleanup goroutine when it exits
	cleared chan struct{} // closed once the cache is closed and emptied
}

// ErrParsePanicked is the error the parses waiting for a parse of the same
// template get when that parse panics.
var ErrParsePanicked = errors.New("astcache: the parse of the template panicked")

type entry struct {
	node    *parser.MJMLNode
	expires time.Time
}

type call struct {
	done chan struct{}
	node *parser.MJMLNode
	err  error
}

// New returns a cache whose entries expire ttl after their parse (DefaultTTL
// if ttl is not positive), and starts its cleanup goroutine.
func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	c := &Cache{
		ttl:     ttl,
		calls:   make(map[string]*call),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
		cleared: make(chan struct{}),
	}
	go c.cleanup(max(ttl/2, time.Millisecond))
	return c
}

// Parse returns the cached tree of template, or parses it with parse and
// stores the result if it succeeds. Errors are not cached. Concurrent calls
// for the same template parse it once; if that parse panics, the others get
// ErrParsePanicked. After Close, it parses every time.
func (c *Cache) Parse(template string, parse func(string) (*parser.MJMLNode, error)) (*parser.MJMLNode, error) {
	if node, ok := c.load(template); ok {
		return node, nil
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return parse(template)
	}
	// Again under the lock: a flight may have stored the entry, and left,
	// since the first look.
	if node, ok := c.load(template); ok {
		c.mu.Unlock()
		return node, nil
	}
	if inFlight, ok := c.calls[template]; ok {
		c.mu.Unlock()
		<-inFlight.done
		return inFlight.node, inFlight.err
	}
	cl := &call{done: make(chan struct{}), err: ErrParsePanicked}
	c.calls[template] = cl
	c.mu.Unlock()

	// The waiters are released even if parse panics: cl.err is still
	// ErrParsePanicked then. The entry is stored before the flight leaves,
	// so that a later caller finds one or the other.
	defer func() {
		c.mu.Lock()
		if cl.err == nil && !c.closed {
			c.entries.Store(template, &entry{node: cl.node, expires: time.Now().Add(c.ttl)})
		}
		delete(c.calls, template)
		c.mu.Unlock()
		close(cl.done)
	}()
	cl.node, cl.err = parse(template)
	return cl.node, cl.err
}

// load returns the unexpired entry of template, removing an expired one.
func (c *Cache) load(template string) (*parser.MJMLNode, bool) {
	v, ok := c.entries.Load(template)
	if !ok {
		return nil, false
	}
	e := v.(*entry)
	if time.Now().Before(e.expires) {
		return e.node, true
	}
	c.entries.CompareAndDelete(template, v)
	return nil, false
}

// Len returns the number of entries, expired ones included until the
// cleanup removes them.
func (c *Cache) Len() int {
	n := 0
	c.entries.Range(func(_, _ any) bool { n++; return true })
	return n
}

// Close stops the cleanup goroutine, waits for it to exit and drops every
// entry. It may be called more than once, also concurrently: every call
// returns once the cache is closed and empty.
func (c *Cache) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		<-c.cleared
		return
	}
	c.closed = true
	close(c.stop)
	c.mu.Unlock()
	<-c.stopped
	c.entries.Clear()
	close(c.cleared)
}

func (c *Cache) cleanup(interval time.Duration) {
	defer close(c.stopped)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			c.entries.Range(func(key, value any) bool {
				if now.After(value.(*entry).expires) {
					c.entries.CompareAndDelete(key, value)
				}
				return true
			})
		case <-c.stop:
			return
		}
	}
}
