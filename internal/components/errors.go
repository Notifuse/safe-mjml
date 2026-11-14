package components

import (
	"errors"

	"github.com/Notifuse/safe-mjml/internal/parser"
)

// NodeError is an error that stops the render at an element of the
// template; the mjml package reports its tag and line.
type NodeError struct {
	TagName string
	Line    int
	Err     error
}

// NewNodeError returns a NodeError for node with the message msg.
func NewNodeError(node *parser.MJMLNode, msg string) *NodeError {
	return WrapNodeError(node, errors.New(msg))
}

// WrapNodeError returns err as a NodeError for node.
func WrapNodeError(node *parser.MJMLNode, err error) *NodeError {
	return &NodeError{TagName: node.GetTagName(), Line: node.GetLineNumber(), Err: err}
}

func (e *NodeError) Error() string { return e.Err.Error() }

func (e *NodeError) Unwrap() error { return e.Err }
