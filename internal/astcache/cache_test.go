package astcache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Notifuse/safe-mjml/internal/parser"
)

const tpl = `<mjml><mj-body><mj-section><mj-column><mj-text>hi</mj-text></mj-column></mj-section></mj-body></mjml>`

// countingParse returns parser.ParseMJML wrapped to count its calls.
func countingParse(calls *atomic.Int32) func(string) (*parser.MJMLNode, error) {
	return func(s string) (*parser.MJMLNode, error) {
		calls.Add(1)
		return parser.ParseMJML(s)
	}
}

func TestCacheStoresAndReusesTree(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	var calls atomic.Int32
	parse := countingParse(&calls)

	first, err := c.Parse(tpl, parse)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Parse(tpl, parse)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 parse, got %d", calls.Load())
	}
	if first != second {
		t.Fatal("expected the cached tree to be reused")
	}
	if c.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", c.Len())
	}
}

func TestCacheSeparateTemplates(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	var calls atomic.Int32
	parse := countingParse(&calls)
	other := `<mjml><mj-body><mj-section><mj-column><mj-text>two</mj-text></mj-column></mj-section></mj-body></mjml>`
	for _, s := range []string{tpl, other, tpl} {
		if _, err := c.Parse(s, parse); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 || c.Len() != 2 {
		t.Fatalf("expected 2 parses and 2 entries, got %d and %d", calls.Load(), c.Len())
	}
}

func TestCacheDoesNotStoreErrors(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	var calls atomic.Int32
	parse := countingParse(&calls)
	for range 2 {
		if _, err := c.Parse("<mjml>", parse); err == nil {
			t.Fatal("expected a parse error")
		}
	}
	if calls.Load() != 2 || c.Len() != 0 {
		t.Fatalf("expected 2 parses and no entry, got %d and %d", calls.Load(), c.Len())
	}
}

func TestCacheExpiration(t *testing.T) {
	c := New(50 * time.Millisecond)
	defer c.Close()
	var calls atomic.Int32
	parse := countingParse(&calls)

	first, _ := c.Parse(tpl, parse)
	time.Sleep(60 * time.Millisecond)
	second, _ := c.Parse(tpl, parse)
	if calls.Load() != 2 {
		t.Fatalf("expected 2 parses after expiration, got %d", calls.Load())
	}
	if first == second {
		t.Fatal("expected a new tree after expiration")
	}
}

func TestCacheHitDoesNotExtendExpiration(t *testing.T) {
	c := New(100 * time.Millisecond)
	defer c.Close()
	var calls atomic.Int32
	parse := countingParse(&calls)
	for i := range 3 {
		if i > 0 {
			time.Sleep(60 * time.Millisecond)
		}
		if _, err := c.Parse(tpl, parse); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("expected expiration without refresh, got %d parses", calls.Load())
	}
}

func TestCacheCleanupRemovesExpiredEntries(t *testing.T) {
	c := New(20 * time.Millisecond)
	defer c.Close()
	var calls atomic.Int32
	if _, err := c.Parse(tpl, countingParse(&calls)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for c.Len() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the cleanup did not remove the expired entry")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestCacheConcurrentParsesParseOnce(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	var calls atomic.Int32
	parse := func(s string) (*parser.MJMLNode, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return parser.ParseMJML(s)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.Parse(tpl, parse); err != nil {
				t.Errorf("parse: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("expected a single parse, got %d", calls.Load())
	}
}

// A parse that panics must release the parses of the same template waiting
// for it with an error, without running them again.
func TestCachePanicReleasesWaitersWithAnError(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer func() { _ = recover() }()
		_, _ = c.Parse(tpl, func(string) (*parser.MJMLNode, error) {
			<-start
			panic("boom")
		})
	}()
	time.Sleep(10 * time.Millisecond) // let the first parse register

	var secondRan atomic.Bool
	var node *parser.MJMLNode
	var err error
	go func() {
		node, err = c.Parse(tpl, func(string) (*parser.MJMLNode, error) {
			secondRan.Store(true)
			return nil, nil
		})
		close(done)
	}()
	time.Sleep(10 * time.Millisecond) // let the second one wait
	close(start)

	select {
	case <-done:
		if secondRan.Load() {
			t.Fatal("the waiting parse should not run")
		}
		if node != nil || err == nil {
			t.Fatalf("the waiting parse got (%v, %v), want an error", node, err)
		}
	case <-time.After(time.Second):
		t.Fatal("the waiting parse was not released after the panic")
	}
}

// Concurrent first parses of a template parse it once: a caller that missed
// the entry before the flight stored it must not start a second flight.
func TestCacheConcurrentMissesParseOnce(t *testing.T) {
	for round := range 200 {
		c := New(time.Minute)
		var calls atomic.Int32
		parse := countingParse(&calls)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for range 64 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if _, err := c.Parse(tpl, parse); err != nil {
					t.Errorf("parse: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
		c.Close()
		if n := calls.Load(); n != 1 {
			t.Fatalf("round %d: %d parses for 64 concurrent callers, want 1", round, n)
		}
	}
}

// Every Close returns once the cache is closed and its entries dropped, not
// only the first one.
func TestCacheConcurrentClose(t *testing.T) {
	for round := range 200 {
		c := New(time.Minute)
		var calls atomic.Int32
		if _, err := c.Parse(tpl, countingParse(&calls)); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		var notEmpty atomic.Int32
		for range 64 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				c.Close()
				if c.Len() != 0 {
					notEmpty.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		if n := notEmpty.Load(); n > 0 {
			t.Fatalf("round %d: %d of 64 Close calls returned before the entries were dropped", round, n)
		}
	}
}

func TestCacheClose(t *testing.T) {
	c := New(time.Minute)
	var calls atomic.Int32
	parse := countingParse(&calls)
	if _, err := c.Parse(tpl, parse); err != nil {
		t.Fatal(err)
	}
	c.Close()
	c.Close() // a second Close does nothing
	select {
	case <-c.stopped:
	default:
		t.Fatal("the cleanup goroutine is still running after Close")
	}
	if c.Len() != 0 {
		t.Fatalf("expected no entry after Close, got %d", c.Len())
	}
	for range 2 {
		if _, err := c.Parse(tpl, parse); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 3 || c.Len() != 0 {
		t.Fatalf("after Close, expected a parse every time and no entry, got %d parses and %d entries", calls.Load(), c.Len())
	}
}

func TestNewWithoutPositiveTTL(t *testing.T) {
	c := New(0)
	defer c.Close()
	if c.ttl != DefaultTTL {
		t.Fatalf("expected the default TTL, got %v", c.ttl)
	}
}
