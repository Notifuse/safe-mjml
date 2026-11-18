package html

import (
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RenderMSOConditional wraps content in MSO/Outlook conditional comments.
// This ensures the content is only rendered in Microsoft Outlook and Internet Explorer,
// while being hidden from other email clients.
//
// The conditional comment format <!--[if mso | IE]>...<![endif]--> is specifically
// recognized by Outlook and IE to provide fallback HTML structures.
//
// Example:
//
//	RenderMSOConditional(w, "<table><tr><td>Outlook content</td></tr></table>")
//	// Output: <!--[if mso | IE]><table><tr><td>Outlook content</td></tr></table><![endif]-->
func RenderMSOConditional(w io.StringWriter, content string) error {
	if _, err := w.WriteString("<!--[if mso | IE]>"); err != nil {
		return err
	}
	if _, err := w.WriteString(content); err != nil {
		return err
	}
	if _, err := w.WriteString("<![endif]-->"); err != nil {
		return err
	}
	return nil
}

const (
	msoConditionalEnd   = "<![endif]-->"
	msoConditionalStart = "<!--[if mso | IE]>"
)

// MergeOutlookConditionalsInBuilder removes, from the document held in sb,
// every "<![endif]-->" followed, after optional whitespace, by
// "<!--[if mso | IE]>", joining adjacent Outlook-only blocks into one. It is
// mjml-core's mergeOutlookConditionnals, which applies
// /(<!\[endif]-->\s*?<!--\[if mso \| IE]>)/gm to the whole document. It copies
// the document once, and not at all when there is nothing to merge.
func MergeOutlookConditionalsInBuilder(sb *strings.Builder) {
	document := sb.String()
	first := firstMergeablePair(document)
	if first < 0 {
		return
	}
	sb.Reset()
	writeMerged(sb, document, first)
}

// MergeOutlookConditionals returns document with its Outlook conditionals
// merged as MergeOutlookConditionalsInBuilder merges them, or document
// itself when there is nothing to merge.
func MergeOutlookConditionals(document string) string {
	first := firstMergeablePair(document)
	if first < 0 {
		return document
	}
	var sb strings.Builder
	writeMerged(&sb, document, first)
	return sb.String()
}

// writeMerged writes document to sb with its Outlook conditionals merged,
// first being the index of its first mergeable pair.
func writeMerged(sb *strings.Builder, document string, first int) {
	sb.Grow(len(document))
	sb.WriteString(document[:first])
	merger := NewOutlookMergeWriter(sb)
	_, _ = merger.WriteString(document[first:]) // a strings.Builder never fails
	_ = merger.Flush()
}

// OutlookPairAt reports whether document has, at offset at, the end of an
// "<![endif]-->" followed, after optional whitespace, by "<!--[if mso | IE]>":
// the pair MergeOutlookConditionalsInBuilder removes. It returns the span to remove,
// from the end marker to the end of the start marker.
func OutlookPairAt(document string, at int) (from, to int, ok bool) {
	if !strings.HasSuffix(document[:at], msoConditionalEnd) {
		return 0, 0, false
	}
	next := skipJSWhitespace(document, at)
	if !strings.HasPrefix(document[next:], msoConditionalStart) {
		return 0, 0, false
	}
	return at - len(msoConditionalEnd), next + len(msoConditionalStart), true
}

// firstMergeablePair returns the index of the first "<![endif]-->" followed,
// after optional whitespace, by "<!--[if mso | IE]>", or -1.
func firstMergeablePair(s string) int {
	offset := 0
	for {
		end := strings.Index(s[offset:], msoConditionalEnd)
		if end < 0 {
			return -1
		}
		end += offset
		next := skipJSWhitespace(s, end+len(msoConditionalEnd))
		if strings.HasPrefix(s[next:], msoConditionalStart) {
			return end
		}
		offset = end + len(msoConditionalEnd)
	}
}

// OutlookMergeWriter applies MergeOutlookConditionalsInBuilder to a stream: it writes
// through to the destination, holding back only text that may belong to a
// "<![endif]-->", whitespace, "<!--[if mso | IE]>" sequence split across
// writes. Call Flush once the document is complete.
type OutlookMergeWriter struct {
	w       io.StringWriter
	held    string
	through bool // merges nothing
}

// NewOutlookMergeWriter returns a writer merging adjacent Outlook
// conditionals before writing to w.
func NewOutlookMergeWriter(w io.StringWriter) *OutlookMergeWriter {
	return &OutlookMergeWriter{w: w}
}

// NewOutlookWriteThrough returns an OutlookMergeWriter that merges nothing
// and writes through to w, for a document whose conditionals are merged
// later, as a whole.
func NewOutlookWriteThrough(w io.StringWriter) *OutlookMergeWriter {
	return &OutlookMergeWriter{w: w, through: true}
}

// WriteString implements io.StringWriter. It reports len(s) on success, since
// merged markers are consumed rather than written.
func (m *OutlookMergeWriter) WriteString(s string) (int, error) {
	if m.through {
		return m.w.WriteString(s)
	}
	n := len(s)
	if m.held == "" && strings.IndexByte(s, '<') < 0 {
		// Every marker starts with '<': this write cannot hold or start one.
		if err := m.write(s); err != nil {
			return 0, err
		}
		return n, nil
	}
	buf := s
	if m.held != "" && len(m.held) < len(msoConditionalEnd) {
		// The previous write ended with a proper prefix of "<![endif]-->"
		// (often a lone "<" written before a tag name).
		need := msoConditionalEnd[len(m.held):]
		switch {
		case strings.HasPrefix(s, need):
			m.held = msoConditionalEnd
			s = s[len(need):]
			buf = s
		case len(s) < len(need) && strings.HasPrefix(need, s):
			m.held += s
			return n, nil
		default:
			if err := m.write(m.held); err != nil {
				return 0, err
			}
			m.held = ""
		}
	}
	switch {
	case m.held == "":
	case m.held == msoConditionalEnd:
		// The common case: the previous write ended with "<![endif]-->".
		// Decide from the start of this write, without concatenating.
		k := skipJSWhitespace(s, 0)
		rest := s[k:]
		switch {
		case strings.HasPrefix(rest, msoConditionalStart):
			m.held = ""
			buf = rest[len(msoConditionalStart):]
		case mayContinueMarker(rest):
			m.held += s
			return n, nil
		default:
			if err := m.write(m.held); err != nil {
				return 0, err
			}
			m.held = ""
		}
	default:
		buf = m.held + s
		m.held = ""
	}
	for {
		end := strings.Index(buf, msoConditionalEnd)
		if end < 0 {
			keep := partialSuffix(buf, msoConditionalEnd)
			if err := m.write(buf[:len(buf)-keep]); err != nil {
				return 0, err
			}
			m.held = buf[len(buf)-keep:]
			return n, nil
		}
		if err := m.write(buf[:end]); err != nil {
			return 0, err
		}
		next := skipJSWhitespace(buf, end+len(msoConditionalEnd))
		rest := buf[next:]
		switch {
		case strings.HasPrefix(rest, msoConditionalStart):
			// A pair to merge: drop the end marker, the whitespace and the start marker.
			buf = rest[len(msoConditionalStart):]
		case mayContinueMarker(rest):
			// The rest may still become a pair with the next write.
			m.held = buf[end:]
			return n, nil
		default:
			if err := m.write(buf[end:next]); err != nil {
				return 0, err
			}
			buf = rest
		}
	}
}

// skipJSWhitespace returns the index of the first byte at or after i that does
// not start a JavaScript whitespace rune (or that starts an incomplete rune).
func skipJSWhitespace(s string, i int) int {
	for i < len(s) {
		if s[i] < utf8.RuneSelf {
			switch s[i] {
			case ' ', '\t', '\n', '\v', '\f', '\r':
				i++
				continue
			}
			return i
		}
		if !utf8.FullRuneInString(s[i:]) {
			return i
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isJSWhitespace(r) {
			return i
		}
		i += size
	}
	return i
}

// mayContinueMarker reports whether rest, found after "<![endif]-->" and its
// whitespace at the end of a write, may still turn into the start marker once
// more text arrives: it is empty, a proper prefix of the start marker, or an
// incomplete rune that may be whitespace.
func mayContinueMarker(rest string) bool {
	if len(rest) < len(msoConditionalStart) && strings.HasPrefix(msoConditionalStart, rest) {
		return true
	}
	return rest != "" && len(rest) < utf8.UTFMax && !utf8.FullRuneInString(rest)
}

// Flush writes the text held back at the end of the document.
func (m *OutlookMergeWriter) Flush() error {
	held := m.held
	m.held = ""
	return m.write(held)
}

func (m *OutlookMergeWriter) write(s string) error {
	if s == "" {
		return nil
	}
	_, err := m.w.WriteString(s)
	return err
}

// partialSuffix returns the length of the longest suffix of s that is a
// proper prefix of marker. marker starts with '<' and contains no other '<'
// (true of "<![endif]-->"), so only the last '<' of s can start one.
func partialSuffix(s, marker string) int {
	tail := s
	if len(tail) > len(marker)-1 {
		tail = tail[len(tail)-(len(marker)-1):]
	}
	i := strings.LastIndexByte(tail, '<')
	if i < 0 || !strings.HasPrefix(marker, tail[i:]) {
		return 0
	}
	return len(tail) - i
}

// isJSWhitespace reports whether r matches \s in a JavaScript regular
// expression: Unicode white space and line terminators, plus the BOM, but not
// U+0085 (NEL), which Go counts as space and JavaScript does not.
func isJSWhitespace(r rune) bool {
	return (unicode.IsSpace(r) && r != '\u0085') || r == '\ufeff'
}
