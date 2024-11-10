package converter

import (
	"bytes"
	"strings"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
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
