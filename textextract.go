package textextract

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

func isInAnchor(n *html.Node) bool {
	if n.Parent == nil {
		return false
	}
	if strings.ToLower(n.Parent.Data) == "a" {
		return true
	}
	return isInAnchor(n.Parent)
}

func normaliseText(t string) string {
	r, _ := regexp.Compile("<[^>]*>|\\n|\\t| +")
	r2, _ := regexp.Compile("^ +| +$")
	return r2.ReplaceAllString(r.ReplaceAllString(
		r.ReplaceAllString(t, " "),
		" "), "")
}

func filter(doc *html.Node, minScore int) *html.Node {
	type NodePair struct {
		Parent *html.Node
		Child  *html.Node
	}
	toDelete := []NodePair{}
	var f func(n *html.Node, score int) int
	f = func(n *html.Node, score int) int {
		if n.Type == html.TextNode {
			count := len(strings.Split(normaliseText(n.Data), " "))
			switch {
			case strings.ToLower(n.Parent.Data) == "script":
			case strings.ToLower(n.Parent.Data) == "style":
			case strings.ToLower(n.Parent.Data) == "link":
			case isInAnchor(n):
				score -= 1 + count ^ 2
			default:
				score += count
			}
			return score
		}

		ownScore := score
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			score += f(c, ownScore)
		}

		if score <= minScore && strings.ToLower(n.Data) != "a" {
			toDelete = append(toDelete, NodePair{n.Parent, n})
		}
		return score
	}
	f(doc, 0)

	for _, x := range toDelete {
		if x.Parent != nil {
			x.Parent.RemoveChild(x.Child)
		}
	}
	return doc
}

const fullStop = "." // Maybe use ".!." for testing to show where auto-added.

func ExtractFromHtml(htmlUTF8Str string, minScore int /*5 is default, -1=>no filter*/, addFullStops bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlUTF8Str))
	if err != nil {
		return "", errors.New("Could not parse HTML string.")
	}
	if minScore >= 0 {
		doc = filter(doc, minScore)
	}
	var f func(n *html.Node)
	var buffer bytes.Buffer
	f = func(n *html.Node) {
		d := normaliseText(n.Data)
		if n.Type == html.TextNode && d != "" && d != " " {
			switch strings.ToLower(n.Parent.Data) {
			case "title":
			case "le":
				if addFullStops && n.Parent.LastChild == n {
					d = strings.TrimSpace(d)
					r, _ := utf8.DecodeLastRuneInString(d)
					if !unicode.IsPunct(r) {
						for _, end := range []string{" &", " and", " /", " or"} { // TODO non-english and/or
							if strings.HasSuffix(d, end) {
								goto noAdd
							}
						}
						d += fullStop
					noAdd:
					}
				}
				buffer.WriteString(fmt.Sprintf("\n%s", d))

			case "h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "p", "th", "td", "figcaption":
				if addFullStops && n.Parent.FirstChild == n {
					if buffer.Len() > 0 {
						r, _ := utf8.DecodeLastRune(buffer.Bytes())
						if !unicode.IsPunct(r) {
							d = fullStop + " " + d
						}
					}
				}
				if addFullStops && n.Parent.LastChild == n {
					d = strings.TrimSpace(d)
					r, _ := utf8.DecodeLastRuneInString(d)
					if !unicode.IsPunct(r) {
						d += fullStop
					}
				}
				buffer.WriteString(fmt.Sprintf("\n%s", d))

			default:
				buffer.WriteString(fmt.Sprintf("\n%s", d))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return buffer.String(), nil
}
