package converter

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/marker"
	"golang.org/x/net/html"
)

const (
	DOUBLE_QUOTE = '"'
	SINGLE_QUOTE = '\''
)

func SurroundBy(content []byte, chars []byte) []byte {
	content = append(chars, content...)
	content = append(content, chars...)
	return content
}

func SurroundByQuotes(content []byte) []byte {
	if len(content) == 0 {
		return nil
	}

	containsDoubleQuote := bytes.ContainsRune(content, DOUBLE_QUOTE)
	containsSingleQuote := bytes.ContainsRune(content, SINGLE_QUOTE)

	if containsDoubleQuote && containsSingleQuote {
		// Escape all quotes
		content = bytes.ReplaceAll(content, []byte(`"`), []byte(`\"`))

		// Surround the content by double quotes
		return SurroundBy(content, []byte(`"`))
	}
	if containsDoubleQuote {
		// Since it contains double quotes (but no single quotes)
		// we can surround it by single quotes
		return SurroundBy(content, []byte(`'`))
	}

	// It may contain single quotes, but definitely no double quotes,
	// so we can safely surround it by double quotes.
	return SurroundBy(content, []byte(`"`))
}

func escapeAlt(altString string) string {
	alt := []byte(altString)

	var buf bytes.Buffer
	for i := range alt {
		if alt[i] == '[' || alt[i] == ']' {
			prevIndex := i - 1
			if prevIndex < 0 || alt[prevIndex] != '\\' {
				buf.WriteRune('\\')
			}
		}
		buf.WriteByte(alt[i])
	}

	return buf.String()
}

func IsImageOrLink(chars []byte, index int) int {
	if chars[index] == '!' {
		return isImageOrLinkStartExclamation(chars, index)
	}
	if chars[index] == '[' {
		return isImageOrLinkStartBracket(chars, index)
	}

	return -1
}

func isImageOrLinkStartExclamation(chars []byte, index int) int {
	nextIndex := index + 1
	if nextIndex < len(chars) && chars[nextIndex] == '[' {
		// It could be the start of an image
		return 1
	}

	return -1
}

func isImageOrLinkStartBracket(chars []byte, index int) int {
	for i := index + 1; i < len(chars); i++ {
		if chars[i] == '\n' {
			return -1
		}

		if chars[i] == ']' {
			return 1
		}
	}

	return -1
}

func TrimConsecutiveNewlines(source []byte) []byte {
	// Some performance optimizations:
	// - If no replacement was done, we return the original slice and dont allocate.
	// - We batch appends

	var ret []byte

	startNormal := 0
	startMatch := -1

	count := 0
	// for i, b := range source {
	for i := 0; i < len(source); i++ {
		r, size := utf8.DecodeRune(source[i:])
		_ = size

		isNewline := r == '\n' || r == marker.MarkerLineBreak
		if isNewline {
			count += 1
		}

		if startMatch == -1 && isNewline {
			// Start of newlines
			startMatch = i
			i = i + size - 1
			continue
		} else if startMatch != -1 && isNewline {
			// Middle of newlines
			i = i + size - 1
			continue
		} else if startMatch != -1 {
			// Character after the last newline character

			if count > 2 {
				if ret == nil {
					ret = make([]byte, 0, len(source))
				}

				ret = append(ret, source[startNormal:startMatch]...)
				ret = append(ret, '\n', '\n')
				startNormal = i
			}

			startMatch = -1
			count = 0
		}
	}

	getStartEnd := func() (int, int, bool, bool) {
		if startMatch == -1 && startNormal == 0 {
			// a) no changes need to be done
			return -1, -1, false, false
		}

		if count <= 2 {
			// b) Only the normal characters still need to be added
			return startNormal, len(source), true, false
		}

		// c) The match still needs to be replaced (and possible the previous normal characters be added)
		return startNormal, startMatch, true, true
	}

	start, end, isKeepNeeded, isReplaceNeeded := getStartEnd()
	if isKeepNeeded {
		if ret == nil {
			ret = make([]byte, 0, len(source))
		}

		ret = append(ret, source[start:end]...)
		if isReplaceNeeded {
			ret = append(ret, '\n', '\n')
		}
	}

	if ret == nil {
		// Huray, we did not do any allocations with make()
		// and instead just return the original slice.
		return source
	}
	return ret
}

var newline = []byte{'\n'}
var escape = []byte{'\\'}

func EscapeMultiLine(content []byte) []byte {
	content = bytes.TrimSpace(content)
	content = TrimConsecutiveNewlines(content)
	if len(content) == 0 {
		return content
	}

	parts := marker.SplitFunc(content, func(r rune) bool {
		return r == '\n' || r == marker.MarkerLineBreak
	})

	for i := range parts {
		parts[i] = bytes.TrimSpace(parts[i])
		if len(parts[i]) == 0 {
			parts[i] = escape
		}
	}
	content = bytes.Join(parts, newline)

	return content
}

func SurroundingSpaces(content []byte) ([]byte, []byte, []byte) {
	rightTrimmed := bytes.TrimRightFunc(content, func(r rune) bool {
		return unicode.IsSpace(r) || r == marker.MarkerLineBreak
	})
	rightExtra := content[len(rightTrimmed):]

	trimmed := bytes.TrimLeftFunc(rightTrimmed, func(r rune) bool {
		return unicode.IsSpace(r) || r == marker.MarkerLineBreak
	})
	leftExtra := content[0 : len(rightTrimmed)-len(trimmed)]

	return leftExtra, trimmed, rightExtra
}

type linkPlugin struct {
	baseURL string
}

// Init implements converter.Plugin.
func (l *linkPlugin) Init(conv *converter.Converter) error {
	conv.Register.UnEscaper(IsImageOrLink, converter.PriorityEarly)
	conv.Register.Renderer(l.handleRender, converter.PriorityEarly)
	return nil
}

// Name implements converter.Plugin.
func (l *linkPlugin) Name() string {
	return "link"
}

func NewLinkPlugin(baseURL string) converter.Plugin {
	return &linkPlugin{
		baseURL: baseURL,
	}
}

func (l *linkPlugin) handleRender(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	name := dom.NodeName(n)

	switch name {
	case "img":
		return l.renderImage(ctx, w, n)
	case "a":
		return l.renderAnchor(ctx, w, n)
	}
	return converter.RenderTryNext

}
func (c *linkPlugin) renderImage(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	src := dom.GetAttributeOr(n, "src", "")
	src = strings.TrimSpace(src)
	if src == "" {
		return converter.RenderTryNext
	}

	src = ctx.AssembleAbsoluteURL(ctx, "img", src)

	if strings.HasPrefix(src, "/") {
		// join the baseURL with the src
		// trim the trailing slash of the baseURL if it exists
		src = strings.TrimSuffix(c.baseURL, "/") + src
	}

	title := dom.GetAttributeOr(n, "title", "")
	title = strings.ReplaceAll(title, "\n", " ")

	alt := dom.GetAttributeOr(n, "alt", "")
	alt = strings.ReplaceAll(alt, "\n", " ")

	// The alt description will be placed between two square brackets `[alt]`
	// so make sure that those characters are escaped.
	alt = escapeAlt(alt)

	w.WriteRune('!')
	w.WriteRune('[')
	w.WriteString(alt)
	w.WriteRune(']')
	w.WriteRune('(')
	w.WriteString(src)
	if title != "" {
		// The destination and title must be seperated by a space
		w.WriteRune(' ')
		w.Write(SurroundByQuotes([]byte(title)))
	}
	w.WriteRune(')')

	return converter.RenderSuccess
}

type link struct {
	*html.Node

	before  []byte
	content []byte
	after   []byte

	href  string
	title string
}

func (c *linkPlugin) renderLinkInlined(w converter.Writer, l *link) converter.RenderStatus {

	w.Write(l.before)
	w.WriteRune('[')
	w.Write(l.content)
	w.WriteRune(']')
	w.WriteRune('(')
	w.WriteString(l.href)
	if l.title != "" {
		// The destination and title must be seperated by a space
		w.WriteRune(' ')
		w.Write(SurroundByQuotes([]byte(l.title)))
	}
	w.WriteRune(')')
	w.Write(l.after)

	return converter.RenderSuccess
}

func (l *linkPlugin) renderAnchor(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	ctx = ctx.WithValue("is_inside_link", true)

	href := dom.GetAttributeOr(n, "href", "")

	href = strings.TrimSpace(href)
	href = ctx.AssembleAbsoluteURL(ctx, "a", href)

	title := dom.GetAttributeOr(n, "title", "")
	title = strings.ReplaceAll(title, "\n", " ")

	li := &link{
		Node:  n,
		href:  href,
		title: title,
	}

	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := buf.Bytes()

	if bytes.TrimFunc(content, marker.IsSpace) == nil {
		// Fallback to the title
		content = []byte(li.title)
	}
	if bytes.TrimSpace(content) == nil {
		return converter.RenderSuccess
	}

	if li.href == "" {
		// A link without href is valid, like e.g. [text]()
		// But a title would make it invalid.
		li.title = ""
	}

	// join the baseURL with the href if the href is relative
	if strings.HasPrefix(li.href, "/") {
		li.href = strings.TrimSuffix(l.baseURL, "/") + li.href
	}

	leftExtra, trimmed, rightExtra := SurroundingSpaces(content)

	trimmed = EscapeMultiLine(trimmed)

	li.before = leftExtra
	li.content = trimmed
	li.after = rightExtra

	return l.renderLinkInlined(w, li)
}
