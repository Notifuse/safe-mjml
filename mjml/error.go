package mjml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/astcache"
	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/juicer"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// ErrorDetail locates the cause of an Error in the template.
type ErrorDetail struct {
	Line    int    `json:"line"`    // the line in the template, 0 if unknown
	Message string `json:"message"` // what is wrong there
	TagName string `json:"tagName"` // the tag of the element, empty for an XML syntax error
}

// Error is the error Render, Compile, Parse and the methods of Document
// return when they fail: Message says what failed and Details, when the
// cause is at a place of the template, its line and tag. Warnings lists the
// validation warnings found before the failure, as a Result would have. It
// has the fields of the error type of github.com/Boostport/mjml-go, and
// Warnings.
//
// In the errors safe-mjml returns, Details is never nil: it marshals as [] when
// there is no place to give. An Error built by the caller without Details
// marshals it as null. When the call panicked, errors.Is(err, ErrInternal)
// reports it and Stack holds the stack of the panic.
type Error struct {
	Message  string        `json:"message"`
	Details  []ErrorDetail `json:"details"`
	Warnings []Warning     `json:"warnings,omitempty"`
	// Stack is set only when the call panicked (errors.Is(err, ErrInternal)):
	// the stack of the goroutine at the panic, for a bug report. It is not
	// in the JSON, nor in Error().
	Stack string `json:"-"`

	cause error
}

// ErrInternal is the cause of the *Error that Render, Compile, Parse and the
// methods of Document return when the call panicked, instead of the panic:
// errors.Is(err, mjml.ErrInternal) reports it. It is a bug of safe-mjml (please
// report it with the template and Error.Stack), or a panic of a function an
// option gave, such as WithIDGenerator's.
var ErrInternal = errors.New("mjml: internal error")

// ErrLimitExceeded is reported by errors.Is for the *Error of a
// compilation that went past one of safe-mjml's limits: the nesting of
// elements, the size of the HTML, or the selectors and the work of the
// inline mj-style and of mj-html-attributes (see Limits).
var ErrLimitExceeded = errors.New("mjml: limit exceeded")

// Is reports whether target is ErrLimitExceeded and the compilation went past
// a limit; errors.Is goes on with the cause.
func (e *Error) Is(target error) bool {
	if target != ErrLimitExceeded {
		return false
	}
	var nesting *parser.NestingError
	return errors.As(e.cause, &nesting) || errors.Is(e.cause, errOutputTooLarge) || errors.Is(e.cause, juicer.ErrLimitExceeded)
}

// internalError is the cause of an *Error made from a panic.
type internalError struct{ value any }

func (e *internalError) Error() string        { return fmt.Sprintf("panic: %v", e.value) }
func (e *internalError) Is(target error) bool { return target == ErrInternal }

// Unwrap returns the value of the panic when it is an error (a runtime
// error, or the error of a concurrent call), for errors.As.
func (e *internalError) Unwrap() error {
	err, _ := e.value.(error)
	return err
}

// recoverInternal, deferred by every exported function that compiles or
// parses, turns a panic of the call into an *Error in *err, so that a bug
// on one template does not end the caller's program.
func recoverInternal(err *error) {
	r := recover()
	if r == nil {
		return
	}
	*err = &Error{
		Message: fmt.Sprintf("mjml: internal error (please report it with the template): %v", r),
		Details: []ErrorDetail{},
		Stack:   string(debug.Stack()),
		cause:   &internalError{value: r},
	}
}

func (e *Error) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)
	if len(e.Details) > 0 {
		sb.WriteString(":")
	}
	for _, detail := range e.Details {
		sb.WriteString("\n- ")
		if detail.Line > 0 {
			sb.WriteString("Line ")
			sb.WriteString(strconv.Itoa(detail.Line))
			if detail.TagName != "" {
				sb.WriteString(" of (")
				sb.WriteString(detail.TagName)
				sb.WriteString(")")
			}
			sb.WriteString(" - ")
		}
		sb.WriteString(detail.Message)
	}
	return sb.String()
}

// Unwrap returns the underlying error, if any, for errors.Is and
// errors.As. Its type (an encoding/xml or internal error) is not part of
// the API and may change: tell errors apart with Message, Details, or
// errors.Is(err, ErrInternal).
func (e *Error) Unwrap() error {
	return e.cause
}

// parseError returns the error of a failed parse as an *Error, with the line
// of an XML syntax error.
func parseError(err error) *Error {
	var nesting *parser.NestingError
	if errors.As(err, &nesting) {
		return &Error{
			Message: "mjml: the template nests its elements too deeply",
			Details: []ErrorDetail{{Line: nesting.Line, Message: nesting.Error(), TagName: nesting.TagName}},
			cause:   err,
		}
	}
	var syntax *xml.SyntaxError
	if errors.As(err, &syntax) {
		return &Error{
			Message: "mjml: the template is not valid XML",
			Details: []ErrorDetail{{Line: syntax.Line, Message: syntax.Msg}},
			cause:   err,
		}
	}
	if errors.Is(err, astcache.ErrParsePanicked) {
		// Another call parsed the template through the same cache and
		// panicked; its own *Error has the stack.
		return &Error{
			Message: "mjml: internal error (please report it with the template): the parse of the template panicked in a concurrent call that shares its cache",
			Details: []ErrorDetail{},
			cause:   &internalError{value: err},
		}
	}
	var malformed *parser.MalformedError
	if errors.As(err, &malformed) {
		// Something after the root element's close tag: as the other
		// malformed MJML errors (checkDocument), with its line and tag.
		return &Error{
			Message: "mjml: cannot render the template",
			Details: []ErrorDetail{{Line: malformed.Line, Message: malformed.Msg, TagName: malformed.TagName}},
			cause:   err,
		}
	}
	if errors.Is(err, io.EOF) {
		// encoding/xml found no element at all: an empty template, or
		// only whitespace and comments.
		return &Error{Message: "mjml: the template has no root element", Details: []ErrorDetail{}, cause: err}
	}
	return &Error{Message: "mjml: cannot parse the template: " + err.Error(), Details: []ErrorDetail{}, cause: err}
}

// renderError returns the error of a failed render as an *Error, with the
// line and tag of the element that caused it.
func renderError(err error) *Error {
	var nodeErr *components.NodeError
	if errors.As(err, &nodeErr) {
		return &Error{
			Message: "mjml: cannot render the template",
			Details: []ErrorDetail{{Line: nodeErr.Line, Message: nodeErr.Err.Error(), TagName: nodeErr.TagName}},
			cause:   err,
		}
	}
	return &Error{Message: "mjml: cannot render the template: " + err.Error(), Details: []ErrorDetail{}, cause: err}
}
