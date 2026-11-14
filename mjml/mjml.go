// Package mjml compiles MJML email templates to responsive HTML, with the
// output of MJML 4.18.
//
// Render returns the HTML of a template; Compile also returns its validation
// warnings, which do not stop the compilation, as with MJML's soft
// validation. Parse parses a template once, to compile it later with
// Document.Compile or Document.Render. Every failure is an *Error.
package mjml

import (
	"time"

	"github.com/Notifuse/safe-mjml/internal/astcache"
	"github.com/Notifuse/safe-mjml/internal/cliopts"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// Render compiles an MJML template to HTML. It is Compile without the
// warnings.
func Render(template string, opts ...Option) (html string, err error) {
	defer recoverInternal(&err)
	res, err := Compile(template, opts...)
	if err != nil {
		return "", err
	}
	return res.HTML, nil
}

// Compile compiles an MJML template to HTML, with the validation warnings.
// A failure is returned as an *Error, a panic too (see ErrInternal).
func Compile(template string, opts ...Option) (res *Result, err error) {
	defer recoverInternal(&err)
	cfg := newConfig(opts)
	ast, err := parse(template, cfg.cache.inner())
	if err != nil {
		return nil, parseError(err)
	}
	return compile(ast, calculateOptimalBufferSize(template), cfg)
}

// Parse parses an MJML template, to compile it later, once or more, with
// Document.Compile or Document.Render. A failure is returned as an *Error,
// a panic too (see ErrInternal): a template that is not well-formed, and one
// that MJML refuses as "Malformed MJML" (a root other than mjml, or no
// mj-body), as Compile would refuse it.
func Parse(template string) (doc *Document, err error) {
	defer recoverInternal(&err)
	ast, err := parse(template, nil)
	if err != nil {
		return nil, parseError(err)
	}
	// What the render would refuse is refused here, so that a template
	// checked with Parse when it is saved can be sent.
	if err := checkDocument(ast); err != nil {
		return nil, renderError(err)
	}
	return &Document{ast: ast, bufferSize: calculateOptimalBufferSize(template)}, nil
}

// Document is a parsed MJML template, made by Parse. It is never modified
// once parsed, so it may be compiled by several goroutines at once. It does
// not keep the template itself.
type Document struct {
	ast        *parser.MJMLNode
	bufferSize int // the size of the output buffer, estimated from the template
}

// Compile compiles the document to HTML, with the validation warnings, as
// Compile does with its template. WithCache has no effect here. A failure,
// or a Document that does not come from Parse, is returned as an *Error, a
// panic too (see ErrInternal).
func (d *Document) Compile(opts ...Option) (res *Result, err error) {
	defer recoverInternal(&err)
	if d == nil || d.ast == nil {
		return nil, &Error{Message: "mjml: the document was not made by Parse", Details: []ErrorDetail{}}
	}
	return compile(d.ast, d.bufferSize, newConfig(opts))
}

// Render compiles the document to HTML. It is Document.Compile without the
// warnings.
func (d *Document) Render(opts ...Option) (html string, err error) {
	defer recoverInternal(&err)
	res, err := d.Compile(opts...)
	if err != nil {
		return "", err
	}
	return res.HTML, nil
}

func compile(ast *parser.MJMLNode, bufferSize int, cfg *config) (*Result, error) {
	html, warnings, err := render(ast, bufferSize, cfg)
	if err != nil {
		mjmlErr := renderError(err)
		if len(warnings) > 0 {
			mjmlErr.Warnings = publicWarnings(warnings)
		}
		return nil, mjmlErr
	}
	return &Result{HTML: html, Warnings: publicWarnings(warnings)}, nil
}

// Result is a compiled template.
type Result struct {
	HTML string `json:"html"`
	// Warnings lists the validation problems of the template (unknown or
	// misplaced tags, invalid attributes), in document order. They do not
	// stop the compilation, as with MJML's default soft validation.
	Warnings []Warning `json:"warnings"`
}

// Warning is a validation problem that does not stop the compilation.
type Warning struct {
	Kind    WarningKind `json:"kind"`
	TagName string      `json:"tagName"` // the tag of the element
	Line    int         `json:"line"`    // the line of the element in the template
	Message string      `json:"message"`
}

// WarningKind classifies a Warning.
type WarningKind string

// Kinds of Warning.
const (
	// WarningUnknownTag: a tag MJML does not know; it is left out.
	WarningUnknownTag WarningKind = "unknown-tag"
	// WarningUnsupportedTag: a tag MJML knows but safe-mjml does not implement
	// (mj-html-attributes, mj-selector, mj-include); it is left out.
	WarningUnsupportedTag WarningKind = "unsupported-tag"
	// WarningMisplacedTag: a known component where MJML does not allow it.
	// As with MJML's soft validation it is rendered all the same by its
	// parent, except directly under the mjml root, which only uses mj-head
	// and mj-body.
	WarningMisplacedTag WarningKind = "misplaced-tag"
	// WarningInvalidAttribute: an attribute the tag does not accept; it is
	// ignored.
	WarningInvalidAttribute WarningKind = "invalid-attribute"
	// WarningInvalidAttributeValue: a value that does not have the type of
	// its attribute, as MJML's soft validation reports it (an enum value it
	// does not list, a length without an accepted unit, a color mjml-core
	// does not know); it is used all the same.
	WarningInvalidAttributeValue WarningKind = "invalid-attribute-value"
)

func publicWarnings(warnings []options.Warning) []Warning {
	out := make([]Warning, len(warnings))
	for i, w := range warnings {
		out[i] = Warning{Kind: WarningKind(w.Kind), TagName: w.TagName, Line: w.Line, Message: w.Message}
	}
	return out
}

// Option configures a compilation.
type Option func(*config)

// config is the configuration of one compilation, which the options set.
// Its fields are unexported, so that another package that recovers it from
// an Option by type inference cannot change it.
type config struct {
	skipComments bool
	idGenerator  func() string
	cache        *Cache
	// internal sets the render options that have no public option, for the
	// commands of this module (internal/cliopts) and the tests.
	internal []func(*options.RenderOpts)
}

// newConfig applies opts, nil ones skipped, to a new configuration.
func newConfig(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

// renderOpts returns the render options of the configuration.
func (c *config) renderOpts() *options.RenderOpts {
	o := &options.RenderOpts{SkipComments: c.skipComments, IDGenerator: c.idGenerator}
	for _, set := range c.internal {
		set(o)
	}
	return o
}

func init() {
	cliopts.New = func(set func(*options.RenderOpts)) any {
		return Option(func(c *config) { c.internal = append(c.internal, set) })
	}
}

// WithKeepComments sets whether the HTML comments of the template are kept in
// the output, as MJML's keepComments option, true by default: each comment
// between components is written where it stands, as an mj-raw would be.
// Comments inside ending tags (mj-text, mj-button, mj-raw…) are content and
// are kept either way.
func WithKeepComments(keep bool) Option {
	return func(c *config) {
		c.skipComments = !keep
	}
}

// WithIDGenerator makes the compilation take the IDs of mj-carousel and of
// the checkbox of an mj-navbar with hamburger from next instead of
// generating random ones, for example to get reproducible output in tests.
// next must return 16 lowercase hex digits, as MJML's IDs have (any other
// value is replaced by a random ID); it is called once per such element, in
// the order the IDs first appear in the output. The generator belongs to the
// caller: share one between concurrent compilations only if it is safe for
// concurrent use.
func WithIDGenerator(next func() string) Option {
	return func(c *config) {
		c.idGenerator = next
	}
}

// WithCache makes Compile and Render take the parsed template from cache, or
// store it there, instead of parsing it every time. A nil cache, or one not
// made by NewCache, compiles without a cache.
func WithCache(cache *Cache) Option {
	return func(c *config) {
		c.cache = cache
	}
}

// Cache keeps parsed templates in memory, for WithCache, so that compiling
// the same template again skips the parse. Make one with NewCache. An entry
// is kept for the cache's TTL from its parse, whether it is used or not;
// there is no size limit, and a background goroutine removes the expired
// entries until Close. Concurrent compilations of the same template parse
// it once. A Cache is safe for concurrent use.
type Cache struct {
	cache *astcache.Cache
}

// NewCache returns a cache whose entries are kept for ttl (5 minutes if ttl
// is not positive). Call Close when it is no longer used.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{cache: astcache.New(ttl)}
}

// Close stops the cache's background goroutine and drops its entries.
// Compilations with the cache then parse their template every time. Close
// on a nil Cache, or one not made by NewCache, does nothing.
func (c *Cache) Close() {
	if inner := c.inner(); inner != nil {
		inner.Close()
	}
}

// inner returns the cache's implementation, nil for a nil Cache or one not
// made by NewCache, which compile without a cache.
func (c *Cache) inner() *astcache.Cache {
	if c == nil {
		return nil
	}
	return c.cache
}
