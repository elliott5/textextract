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
	"golang.org/x/net/html/atom"
)

func isInAnchor(n *html.Node) bool {
	if n.Parent == nil {
		return false
	}
	if n.Parent.DataAtom == atom.A {
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
			switch n.Parent.DataAtom {
			case atom.Script, atom.Style, atom.Link: // ignore
			default:
				if isInAnchor(n) {
					score -= 1 + count ^ 2
				} else {
					score += count
				}
			}
			return score
		}

		ownScore := score
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			score += f(c, ownScore)
		}

		if score <= minScore && n.DataAtom != atom.A {
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

func ExtractFromHtml(htmlUTF8Str string, minScore int /*5 is default, -1=>no filter*/, addFullStops bool, lang string) (string, string, error) {
	if len(lang) < 2 {
		return "", "", errors.New("language not supported: " + lang)
	}
	listEndings, langFound := map[string][]string{
		"en": []string{"&", "and", "/", "or"}, // TODO non-english and/or
	}[strings.ToLower(lang[:2])]
	if !langFound {
		return "", "", errors.New("language not supported: " + lang)
	}

	doc, err := html.Parse(strings.NewReader(htmlUTF8Str))
	if err != nil {
		return "", "", err
	}
	if minScore >= 0 {
		doc = filter(doc, minScore)
	}
	var f func(n *html.Node)
	var buffer bytes.Buffer
	var title string
	f = func(n *html.Node) {
		d := normaliseText(n.Data)
		if n.Type == html.TextNode && d != "" && d != " " {
			switch n.Parent.DataAtom {
			case atom.Title: // don't pass the title through
				title = d
			case atom.Li:
				if addFullStops && n.Parent.LastChild == n {
					d = strings.TrimSpace(d)
					r, _ := utf8.DecodeLastRuneInString(d)
					if !unicode.IsPunct(r) {
						for _, end := range listEndings {
							if strings.HasSuffix(d, end) {
								prevRune, _ := utf8.DecodeLastRuneInString(strings.TrimSuffix(d, end))
								if unicode.IsPunct(prevRune) || unicode.IsSpace(prevRune) {
									goto noAdd
								}
							}
						}
						d += fullStop
					noAdd:
					}
				}
				buffer.WriteString(fmt.Sprintf("\n%s", d))

			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.P, atom.Th, atom.Td, atom.Figcaption:
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
	return title, buffer.String(), nil
}
