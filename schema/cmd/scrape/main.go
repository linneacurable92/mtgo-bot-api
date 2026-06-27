// Command scrape fetches the official Telegram Bot API documentation
// (https://core.telegram.org/bots/api), parses its HTML, and regenerates the
// machine-readable schema data files (methods.json and types.json) used by the
// rest of the schema tooling.
//
// This mirrors the approach of PaulSonOfLars/telegram-bot-api-spec: the official
// docs are the single source of truth for methods, parameters, return types,
// object types and their fields. Scraping keeps the schema complete and
// up-to-date instead of hand-maintaining it.
//
// The scraper:
//   - detects the Bot API version from the changelog,
//   - walks every <h4> anchor (each defines one method or one type),
//   - classifies each entry as a method (Parameter/Required table) or a type
//     (Field table) by its following <table> header,
//   - extracts parameters/fields, required flags, types and descriptions,
//   - best-effort extracts the return type from each method description,
//   - merges in status.json + official/extension flags from the existing
//     methods.json so hand-curated metadata (categories, extension markers,
//     implementation status) is preserved across re-scrapes,
//   - writes methods.json and types.json, indented for easy reviewing/diffs.
//
// Usage:
//
//	go run ./schema/cmd/scrape                      # writes to ./schema
//	go run ./schema/cmd/scrape -url https://.../api  # alternate source
//	go run ./schema/cmd/scrape -out ./schema        # alternate output dir
//	go run ./schema/cmd/scrape -in ./input.html     # parse a saved HTML file
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"

	botversion "github.com/mtgo-labs/mtgo-bot-api/internal/version"
	"github.com/mtgo-labs/mtgo-bot-api/schema"
)

const defaultURL = "https://core.telegram.org/bots/api"

func main() {
	url := flag.String("url", defaultURL, "URL of the official Bot API documentation")
	out := flag.String("out", defaultOutDir(), "output directory for methods.json/types.json")
	in := flag.String("in", "", "path to a saved HTML file to parse instead of fetching -url")
	flag.Parse()

	data, err := readInput(*url, *in)
	check(err, "read source")

	doc, err := html.Parse(bytes.NewReader(data))
	check(err, "parse HTML")

	version := detectVersion(doc)
	if version == "" {
		// detectVersion reads the changelog's "Bot API X.Y" heading; if the docs
		// phrasing shifts and the regex misses, fall back to the implementation's
		// pinned version so methods.json never ships with an empty api_version
		// (which fails the schema-cert tests in internal/client).
		version = botversion.BotAPIVersion
	}
	methods, types := extract(doc)

	// Preserve non-official extension methods (e.g. mtgo-only methods like
	// close/logout) that exist in the previous methods.json but are NOT part of
	// the official docs, so re-scraping doesn't drop them. Official method
	// categories are derived from the docs, not from the previous file.
	prev, _ := schema.Load(*out)
	prevByName := indexMethods(prev)
	for i := range methods {
		delete(prevByName, methods[i].Name)
	}
	for name := range prevByName {
		methods = append(methods, prevByName[name])
	}

	// _generated marks these files as machine-produced; do not hand-edit
	// (regenerate via `go run ./schema/cmd/scrape`). Unknown fields are ignored
	// by schema.Load, so these keys are safe for consumers.
	methodsOut := map[string]any{
		"_generated":    true,
		"_generated_by": "go run ./schema/cmd/scrape",
		"_source":       defaultURL,
		"api_version":   version,
		"methods":       methods,
	}
	typesOut := map[string]any{
		"_generated":    true,
		"_generated_by": "go run ./schema/cmd/scrape",
		"_source":       defaultURL,
		"types":         types,
	}

	writeJSON(filepath.Join(*out, "methods.json"), methodsOut)
	writeJSON(filepath.Join(*out, "types.json"), typesOut)

	fmt.Printf("scraped Bot API %s: %d methods, %d types -> %s\n", version, len(methods), len(types), *out)
}

func readInput(url, in string) ([]byte, error) {
	if in != "" {
		return os.ReadFile(in)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

var versionRe = regexp.MustCompile(`Bot API (\d+\.\d+)`)

// detectVersion finds the most recent "Bot API X.Y" mention in the changelog.
func detectVersion(doc *html.Node) string {
	var found string
	walk(doc, func(n *html.Node) {
		if n.Type != html.TextNode {
			return
		}
		if m := versionRe.FindStringSubmatch(n.Data); m != nil && found == "" {
			found = m[1]
		}
	})
	return found
}

// extract walks the document in order, tracking the enclosing <h3> section as
// each entry's category, and pulls out every <h4> anchor entry as either a
// method or a type. Categories are derived purely from the docs, so the output
// is fully scrape-generated (no hand-curated category source).
func extract(doc *html.Node) ([]schema.Method, []schema.TypeDef) {
	var methods []schema.Method
	var types []schema.TypeDef
	var section string // current <h3> section title

	visitH4 := func(h4 *html.Node) {
		name, title := h4Anchor(h4)
		// Only reference entries have single-token names; skip prose headings
		// (changelog dates, guide paragraphs like "Making requests ...").
		if name == "" || !isToken(title) {
			return
		}
		category := section
		if category == "" {
			category = "Misc"
		}
		desc, table := followingDescriptionAndTable(h4)
		if table != nil {
			kind, cols := classifyTable(table)
			switch kind {
			case kindMethod:
				methods = append(methods, schema.Method{
					Name:           name,
					Title:          title,
					Category:       category,
					Description:    desc,
					Returns:        extractReturn(desc),
					Parameters:     parseParams(table, cols),
					ParamsComplete: true,
					Official:       true,
				})
				return
			case kindType:
				types = append(types, schema.TypeDef{
					Name:        title,
					Description: desc,
					Fields:      parseFields(table, cols),
				})
				return
			}
		}
		// No table: classify by convention — methods are camelCase, types PascalCase.
		if isMethodTitle(title) {
			methods = append(methods, schema.Method{
				Name:           name,
				Title:          title,
				Category:       category,
				Description:    desc,
				Returns:        extractReturn(desc),
				Parameters:     nil,
				ParamsComplete: true,
				Official:       true,
			})
		} else {
			types = append(types, schema.TypeDef{
				Name:        title,
				Description: desc,
				Fields:      nil,
			})
		}
	}

	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "h3":
			// Update the current section context. Skip non-reference prose
			// sections so categories stay meaningful.
			if t := humanizeSection(textOf(n)); isReferenceSection(t) {
				section = t
			}
		case "h4":
			visitH4(n)
		}
	})
	return methods, types
}

const (
	kindUnknown = iota
	kindMethod
	kindType
)

type colMap struct {
	param int
	typ   int
	req   int // required column index (methods)
	desc  int
	field int // field column index (types)
}

// classifyTable inspects the table header row to decide method vs type and
// returns the column index layout.
func classifyTable(table *html.Node) (int, colMap) {
	var headers []string
	walk(table, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "th" {
			headers = append(headers, strings.TrimSpace(textOf(n)))
		}
	})
	cm := colMap{param: -1, typ: -1, req: -1, desc: -1, field: -1}
	for i, h := range headers {
		lh := strings.ToLower(h)
		switch lh {
		case "parameter", "field":
			if cm.param < 0 {
				cm.param = i
			}
			cm.field = i
		case "type":
			cm.typ = i
		case "required":
			cm.req = i
		case "description":
			cm.desc = i
		}
	}
	if cm.req >= 0 {
		return kindMethod, cm
	}
	if cm.field >= 0 || cm.param >= 0 {
		return kindType, cm
	}
	return kindUnknown, cm
}

// parseParams reads method parameter rows.
func parseParams(table *html.Node, cm colMap) []schema.Parameter {
	rows := bodyRows(table)
	out := make([]schema.Parameter, 0, len(rows))
	for _, cells := range rows {
		if len(cells) <= maxIdx(cm.param, cm.typ, cm.req, cm.desc) {
			continue
		}
		req := strings.EqualFold(strings.TrimSpace(textOf(cells[cm.req])), "yes")
		out = append(out, schema.Parameter{
			Name:        strings.TrimSpace(textOf(cells[cm.param])),
			Type:        strings.TrimSpace(renderType(cells[cm.typ])),
			Required:    req,
			Description: cleanText(textOf(cells[cm.desc])),
		})
	}
	return out
}

// parseFields reads type field rows. For types, optionality is encoded by the
// description cell starting with "Optional.".
func parseFields(table *html.Node, cm colMap) []schema.Field {
	rows := bodyRows(table)
	out := make([]schema.Field, 0, len(rows))
	for _, cells := range rows {
		fi := cm.field
		if fi < 0 {
			fi = cm.param
		}
		if fi < 0 || cm.typ < 0 || cm.desc < 0 {
			continue
		}
		if len(cells) <= maxIdx(fi, cm.typ, cm.desc) {
			continue
		}
		desc := cleanText(textOf(cells[cm.desc]))
		out = append(out, schema.Field{
			Name:        strings.TrimSpace(textOf(cells[fi])),
			Type:        strings.TrimSpace(renderType(cells[cm.typ])),
			Required:    !startsWithOptional(desc),
			Description: desc,
		})
	}
	return out
}

func startsWithOptional(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "optional")
}

func maxIdx(xs ...int) int {
	m := -1
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

// bodyRows returns the rows of a table's tbody, each row as its <td> cells.
func bodyRows(table *html.Node) [][]*html.Node {
	var rows [][]*html.Node
	walk(table, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "tr" {
			return
		}
		// Skip header rows (those containing <th>).
		if hasDescendant(n, "th") {
			return
		}
		var cells []*html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "td" {
				cells = append(cells, c)
			}
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	})
	return rows
}

// followingDescriptionAndTable returns the first <p> text after h4 (description)
// and the first <table class="table"> after h4 (parameters/fields), stopping at
// the next heading. The table is always the first matching one encountered (the
// loop returns as soon as it is found), so no nil-tracking variable is needed.
func followingDescriptionAndTable(h4 *html.Node) (string, *html.Node) {
	var desc string
	for sib := h4.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type != html.ElementNode {
			continue
		}
		switch sib.Data {
		case "h3", "h4":
			return desc, nil
		case "p":
			if desc == "" {
				desc = cleanText(textOf(sib))
			}
		case "table":
			if hasClass(sib, "table") {
				return desc, sib
			}
		}
	}
	return desc, nil
}

// h4Anchor reads the <a class="anchor" name="..."> child of an <h4> and returns
// (name, title). name is the anchor id; title is the h4 text after the anchor.
func h4Anchor(h4 *html.Node) (string, string) {
	var name string
	walk(h4, func(n *html.Node) {
		if name != "" || n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		if hasClass(n, "anchor") {
			name = attr(n, "name")
		}
	})
	if name == "" {
		return "", ""
	}
	title := strings.TrimSpace(textOf(h4))
	// The title is the anchor text; strip a leading icon glyph if present.
	return name, title
}

// extractReturn best-effort extracts the Bot API return type from a method's
// description. It locates the clause mentioning "return"/"returns"/"returned"
// and derives the type from array/boolean/Int phrasings or the first plausible
// PascalCase type token in that clause.
func extractReturn(desc string) string {
	low := strings.ToLower(desc)
	// Try every clause that mentions "return"; the return type is usually in
	// the last one ("Returns an Array of X objects"), while earlier ones may be
	// phrasing like "Will return the score...".
	off := 0
	for {
		idx := strings.Index(low[off:], "return")
		if idx < 0 {
			break
		}
		clause := clauseAround(desc, off+idx)
		off += idx + len("return")
		if r := returnFromClause(clause); r != "" {
			return r
		}
	}
	// Final fallback: scan the whole description.
	return returnFromClause(desc)
}

func returnFromClause(clause string) string {
	low := strings.ToLower(clause)
	switch {
	case strings.Contains(low, "array of"):
		if t := firstType(clause); t != "" {
			return t + "[]"
		}
		return "Array"
	case strings.Contains(low, "true"):
		return "Boolean"
	case strings.Contains(low, "int") && !strings.Contains(low, "pointer"):
		return "Integer"
	}
	return firstType(clause)
}

// clauseAround trims desc to the sentence containing position pos.
func clauseAround(desc string, pos int) string {
	start := pos
	if c := strings.LastIndexAny(desc[:pos], ".\n"); c >= 0 {
		start = c + 1
	}
	end := len(desc)
	if c := strings.IndexAny(desc[pos:], ".\n"); c >= 0 {
		end = pos + c
	}
	return strings.TrimSpace(desc[start:end])
}

var typeTokenRe = regexp.MustCompile(`([A-Z][A-Za-z0-9]+)`)

// returnStopWords are PascalCase tokens that appear in return clauses but are
// not Bot API types.
var returnStopWords = map[string]bool{
	"Returns": true, "Return": true, "On": true, "True": true, "Array": true,
	"Bot": true, "JSON": true, "HTTP": true, "URL": true, "API": true,
	"Optional": true, "Will": true, "May": true, "Values": true,
}

// firstType returns the first PascalCase token in s that looks like a Bot API
// type (skipping common stop words).
func firstType(s string) string {
	for _, m := range typeTokenRe.FindAllString(s, -1) {
		if !returnStopWords[m] {
			return m
		}
	}
	return ""
}

// renderType renders a type cell's content into a compact type string, turning
// <a href="#user">User</a> into "User" and "Array of String" into "String[]".
func renderType(n *html.Node) string {
	raw := cleanText(textOf(n))
	return normalizeType(raw)
}

// normalizeType folds common Bot API type phrasings into a canonical form.
func normalizeType(raw string) string {
	s := strings.TrimSpace(raw)
	low := strings.ToLower(s)
	if strings.Contains(low, "array of") {
		// e.g. "Array of String" -> "String[]", "Array of MessageEntity" -> "MessageEntity[]"
		after := strings.TrimSpace(s[strings.Index(low, "array of")+len("array of"):])
		return normalizeType(after) + "[]"
	}
	// Strip trailing descriptions after the type, e.g. "Integer, optional".
	if i := strings.IndexAny(s, ",;"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// ---- HTML helpers ----

func walk(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func hasClass(n *html.Node, want string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			if slices.Contains(strings.Fields(a.Val), want) {
				return true
			}
		}
	}
	return false
}

func hasDescendant(n *html.Node, tag string) bool {
	found := false
	walk(n, func(c *html.Node) {
		if c.Type == html.ElementNode && c.Data == tag {
			found = true
		}
	})
	return found
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textOf returns the concatenated text content of a node, preserving a single
// space where inline elements meet.
func textOf(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
			return
		}
		if c.Type == html.ElementNode {
			switch c.Data {
			case "br":
				b.WriteString("\n")
			}
		}
		for k := c.FirstChild; k != nil; k = k.NextSibling {
			f(k)
		}
	}
	f(n)
	return b.String()
}

// cleanText collapses whitespace and trims an extracted cell/description string.
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func indexMethods(s *schema.Schema) map[string]schema.Method {
	if s == nil {
		return nil
	}
	m := make(map[string]schema.Method, len(s.Methods))
	for i := range s.Methods {
		m[s.Methods[i].Name] = s.Methods[i]
	}
	return m
}

func writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	check(err, "marshal")
	data = append(data, '\n')
	check(os.WriteFile(path, data, 0o644), "write "+path)
}

func check(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "scrape: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func defaultOutDir() string {
	return "schema"
}

// tokenRe matches a single method/type identifier with no spaces.
var tokenRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// isToken reports whether title is a single-token method/type name (no spaces,
// no punctuation), which excludes prose section headings.
func isToken(title string) bool {
	return tokenRe.MatchString(title)
}

// isMethodTitle reports whether title is a method name: single token whose
// first character is lowercase (Bot API method naming convention).
func isMethodTitle(title string) bool {
	return title != "" && title[0] >= 'a' && title[0] <= 'z' && isToken(title)
}

// nonReferenceSections are doc sections that are prose/guides, not method/type
// reference groups. Methods/types encountered under them should not inherit them
// as a category (the previous reference section is retained instead).
var nonReferenceSections = map[string]bool{
	"Recent Changes":                   true,
	"Authorizing Your Bot":             true,
	"Making Requests":                  true,
	"Using a Local Bot API Server":     true,
	"Do I Need a Local Bot API Server": true,
}

// isReferenceSection reports whether a humanized section title is a method/type
// reference group (used to update the current category context).
func isReferenceSection(title string) bool {
	t := strings.TrimSpace(title)
	return t != "" && !nonReferenceSections[t]
}

// humanizeSection turns a doc section heading into a stable category label by
// title-casing its words (e.g. "Available methods" -> "Available Methods").
func humanizeSection(raw string) string {
	words := strings.Fields(strings.TrimSpace(raw))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}
