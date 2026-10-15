package subtpl

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"remnawave-go/internal/subtpl/ojson"
)

// DumpYAML ports js-yaml 5's dump() with default options (DUMP_SCHEMA,
// indent 2, lineWidth 80, single-quote preference, block collections).
// Anchors for repeated object references are not produced: the generators
// never share objects in their output.
func DumpYAML(value any) (string, error) {
	root := buildYAML(value)
	if root == nil {
		return "", nil
	}
	d := &yamlDumper{}
	text, _ := d.writeNode(0, root, nil, writeContext{block: true, compact: true})
	out := text + "\n"
	if d.openEnded {
		out += "...\n"
	}
	return out, nil
}

const (
	nodeScalar = iota
	nodeSequence
	nodeMapping
)

const (
	stylePlain = 1 + iota
	styleSingle
	styleDouble
	styleLiteral
	styleFolded
)

type yamlNode struct {
	kind  int
	tag   string // scalar tag short name: str, null, bool, int, float
	value string
	items []*yamlNode
	keys  []*yamlNode
}

// buildYAML ports jsToAst for ojson values; nil means an unrepresentable
// value (undefined).
func buildYAML(value any) *yamlNode {
	switch v := value.(type) {
	case nil:
		return &yamlNode{kind: nodeScalar, tag: "null", value: "null"}
	case bool:
		return &yamlNode{kind: nodeScalar, tag: "bool", value: strconv.FormatBool(v)}
	case float64:
		return yamlNumber(v)
	case string:
		return &yamlNode{kind: nodeScalar, tag: "str", value: v}
	case *ojson.Object:
		if v == nil {
			return &yamlNode{kind: nodeScalar, tag: "null", value: "null"}
		}
		node := &yamlNode{kind: nodeMapping}
		for _, key := range v.Keys() {
			item := buildYAML(v.Value(key))
			if item == nil {
				continue
			}
			node.keys = append(node.keys, &yamlNode{kind: nodeScalar, tag: "str", value: key})
			node.items = append(node.items, item)
		}
		return node
	case []any:
		node := &yamlNode{kind: nodeSequence}
		for _, entry := range v {
			item := buildYAML(entry)
			if item == nil {
				item = buildYAML(nil)
			}
			node.items = append(node.items, item)
		}
		return node
	}
	if value == ojson.Undefined {
		return nil
	}
	return buildYAML(ojson.FromGo(value))
}

func yamlNumber(v float64) *yamlNode {
	text := ojson.FormatNumber(v)
	negativeZero := v == 0 && math.Signbit(v)
	if v == math.Trunc(v) && !math.IsInf(v, 0) && !negativeZero && !strings.Contains(text, "e") {
		return &yamlNode{kind: nodeScalar, tag: "int", value: text}
	}
	switch {
	case math.IsNaN(v):
		text = ".nan"
	case math.IsInf(v, 1):
		text = ".inf"
	case math.IsInf(v, -1):
		text = "-.inf"
	case negativeZero:
		text = "-0.0"
	case jsExponentInteger.MatchString(text):
		text = strings.Replace(text, "e", ".e", 1)
	}
	return &yamlNode{kind: nodeScalar, tag: "float", value: text}
}

var jsExponentInteger = regexp.MustCompile(`^[-+]?[0-9]+e`)

type writeContext struct {
	block, compact, isKey bool
}

type yamlDumper struct {
	openEnded bool
}

const yamlIndent = 2

func nextLine(level int) string {
	return "\n" + strings.Repeat(" ", yamlIndent*level)
}

func (d *yamlDumper) writeNode(level int, node, parent *yamlNode, ctx writeContext) (string, bool) {
	useBlock := ctx.block && node.kind != nodeScalar && len(node.items) > 0
	var body string
	printTag := false
	switch node.kind {
	case nodeMapping:
		if useBlock {
			body = d.writeBlockMapping(level, node, ctx.compact)
		} else {
			body = d.writeFlowMapping(level, node)
		}
	case nodeSequence:
		if useBlock {
			body = d.writeBlockSequence(level, node, ctx.compact)
		} else {
			body = d.writeFlowSequence(level, node)
		}
	default:
		layout := newScalarLayout(node, level, ctx.isKey, !ctx.block)
		layout.applyRules()
		body = layout.render()
		d.openEnded = (layout.style == styleLiteral || layout.style == styleFolded) && (node.value == "\n" || strings.HasSuffix(node.value, "\n\n"))
		printTag = (body == "" && layout.flowOnly && parent != nil && parent.kind == nodeSequence) || (layout.style != stylePlain && node.tag != "str")
	}
	if node.kind != nodeScalar && !useBlock {
		d.openEnded = false
	}
	noBody := body == ""
	if printTag {
		separator := " "
		if body == "" || body[0] == '\n' {
			separator = ""
		}
		body = "!!" + node.tag + separator + body
	}
	return body, noBody
}

func (d *yamlDumper) writeFlowSequence(level int, node *yamlNode) string {
	parts := make([]string, len(node.items))
	for i, item := range node.items {
		parts[i], _ = d.writeNode(level, item, node, writeContext{})
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (d *yamlDumper) writeBlockSequence(level int, node *yamlNode, compact bool) string {
	result := ""
	for _, item := range node.items {
		text, _ := d.writeNode(level+1, item, node, writeContext{block: true, compact: true})
		if !compact || result != "" {
			result += nextLine(level)
		}
		if text == "" || text[0] == '\n' {
			result += "-"
		} else {
			result += "- "
		}
		result += text
	}
	return result
}

func (d *yamlDumper) writeFlowMapping(level int, node *yamlNode) string {
	result := ""
	for i, key := range node.keys {
		if result != "" {
			result += ", "
		}
		keyText, _ := d.writeNode(level, key, node, writeContext{isKey: true})
		valueText, _ := d.writeNode(level, node.items[i], node, writeContext{})
		separator := " "
		if valueText == "" {
			separator = ""
		}
		result += keyText + ":" + separator + valueText
	}
	return "{" + result + "}"
}

func (d *yamlDumper) writeBlockMapping(level int, node *yamlNode, compact bool) string {
	result := ""
	for i, key := range node.keys {
		pair := ""
		if !compact || result != "" {
			pair += nextLine(level)
		}
		keyText, _ := d.writeNode(level+1, key, node, writeContext{block: true, compact: true, isKey: true})
		keyTooLong := utf16Len(keyText) > 1024 && utf8.RuneCountInString(keyText) >= 1025
		explicit := strings.Contains(key.value, "\n") || keyTooLong
		if explicit {
			if keyText != "" && keyText[0] == '\n' {
				pair += "?"
			} else {
				pair += "? "
			}
		}
		pair += keyText
		if explicit {
			pair += nextLine(level)
		}
		valueText, _ := d.writeNode(level+1, node.items[i], node, writeContext{block: true, compact: explicit})
		if valueText == "" || valueText[0] == '\n' {
			pair += ":"
		} else {
			pair += ": "
		}
		result += pair + valueText
	}
	return result
}

// scalarLayout ports the presenter's scalar layout and styling rules.
type scalarLayout struct {
	node                                            *yamlNode
	isKey, flowOnly                                 bool
	shiftOfParent, shiftOfContent, shiftOfFirstLine int
	allowed                                         int
	style                                           int
}

func newScalarLayout(node *yamlNode, level int, isKey, flowOnly bool) *scalarLayout {
	l := &scalarLayout{node: node, isKey: isKey, flowOnly: flowOnly, style: stylePlain}
	if level == 0 {
		l.shiftOfParent, l.shiftOfFirstLine = -1, 0
	} else {
		l.shiftOfParent, l.shiftOfFirstLine = yamlIndent*(level-1), yamlIndent*level
	}
	l.shiftOfContent = yamlIndent * max(1, level)
	l.allowed = 1 << styleDouble
	if l.canUsePlain() {
		l.allowed |= 1 << stylePlain
	}
	if l.canUseSingle() {
		l.allowed |= 1 << styleSingle
	}
	if l.canUseBlock() {
		l.allowed |= 1<<styleLiteral | 1<<styleFolded
	}
	return l
}

func (l *scalarLayout) allows(style int) bool { return l.allowed&(1<<style) != 0 }

var (
	invisiblePattern  = regexp.MustCompile(`[\t\x{7F}-\x{A0}\x{2028}\x{2029}\x{FEFF}\x{FFFE}\x{FFFF}]`)
	forbiddenFirst    = regexp.MustCompile(`^(?:---|\.\.\.)(?:$|[ \t\n\r])`)
	forbiddenContent  = regexp.MustCompile(`(?m)^(?:---|\.\.\.)(?:$|[ \t\n\r])`)
	spaceAroundBreak  = regexp.MustCompile("[ \t]\n|\n[ \t]")
	foldablePattern   = regexp.MustCompile(" [^ \t]")
	leadingSpaceBlock = regexp.MustCompile(`^\n* `)
)

func (l *scalarLayout) canUsePlain() bool {
	value := l.node.value
	if value != "" {
		if !plainScalar(value, !l.isKey) {
			return false
		}
		if l.shiftOfFirstLine == 0 && forbiddenFirst.MatchString(value) {
			return false
		}
	}
	resolved := implicitTag(value)
	if resolved != l.node.tag {
		return false
	}
	return !(value == "=" && resolved == "str")
}

func (l *scalarLayout) canUseSingle() bool {
	value := l.node.value
	for _, r := range value {
		if r == '\n' && !l.isKey {
			continue
		}
		if !(r == '\t' || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFF || r >= 0x10000) {
			return false
		}
	}
	return !spaceAroundBreak.MatchString(value)
}

func (l *scalarLayout) canUseBlock() bool {
	if l.flowOnly {
		return false
	}
	for _, r := range l.node.value {
		if r != '\n' && !(isPrintable(r) && r != '\r' && r != 0xFEFF) {
			return false
		}
	}
	contentIndent := l.shiftOfContent - l.shiftOfParent
	if contentIndent < 1 || contentIndent > 9 && leadingSpaceBlock.MatchString(l.node.value) {
		return false
	}
	return !(l.shiftOfContent == 0 && forbiddenContent.MatchString(l.node.value))
}

func isJSWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

func (l *scalarLayout) availableWidth() int {
	return max(40, 80-l.shiftOfContent)
}

// applyRules runs DEFAULT_SCALAR_STYLE_RULES in order.
func (l *scalarLayout) applyRules() {
	value := l.node.value
	if l.style == stylePlain && invisiblePattern.MatchString(value) {
		l.style = styleDouble
	}
	if l.style == stylePlain && value != "" && strings.IndexFunc(value, func(r rune) bool { return !isJSWhitespace(r) }) == -1 {
		l.style = styleDouble
	}
	if l.style == stylePlain && !l.isKey {
		multiline := strings.Contains(value, "\n")
		if !l.allows(styleLiteral) {
			if multiline {
				l.style = styleDouble
			}
		} else {
			fold := false
			for _, line := range strings.Split(value, "\n") {
				if utf16Len(line) > l.availableWidth() && !strings.HasPrefix(line, " ") && foldablePattern.MatchString(line) {
					fold = true
				}
			}
			if fold {
				l.style = styleFolded
			} else if multiline {
				l.style = styleLiteral
			}
		}
	}
	if l.style == stylePlain && !l.allows(stylePlain) {
		if l.allows(styleSingle) {
			l.style = styleSingle
		} else {
			l.style = styleDouble
		}
	}
	if !l.allows(l.style) {
		l.style = styleDouble
	}
}

func (l *scalarLayout) render() string {
	value := l.node.value
	switch l.style {
	case stylePlain:
		return encodeFlowBreaks(value, l.shiftOfContent)
	case styleSingle:
		return "'" + strings.ReplaceAll(encodeFlowBreaks(value, l.shiftOfContent), "'", "''") + "'"
	case styleLiteral:
		return "|" + blockHeader(value, l.shiftOfParent, l.shiftOfContent) + dropEndingNewline(indentLines(value, l.shiftOfContent))
	case styleFolded:
		return ">" + blockHeader(value, l.shiftOfParent, l.shiftOfContent) + dropEndingNewline(indentLines(foldBlockScalar(value, l.availableWidth()), l.shiftOfContent))
	}
	return `"` + escapeYAMLString(value) + `"`
}

var lineBreakRun = regexp.MustCompile(`(\n+)([^\n]*)`)

func encodeFlowBreaks(s string, shift int) string {
	first := strings.IndexByte(s, '\n')
	if first == -1 {
		return s
	}
	pad := strings.Repeat(" ", shift)
	result := s[:first]
	for _, match := range lineBreakRun.FindAllStringSubmatch(s[first:], -1) {
		result += strings.Repeat("\n", len(match[1])+1) + pad + match[2]
	}
	return result
}

func indentLines(s string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	var b strings.Builder
	for len(s) > 0 {
		line := s
		if next := strings.IndexByte(s, '\n'); next != -1 {
			line = s[:next+1]
		}
		s = s[len(line):]
		if line != "" && line != "\n" {
			b.WriteString(indent)
		}
		b.WriteString(line)
	}
	return b.String()
}

func blockHeader(s string, shiftOfParent, shiftOfContent int) string {
	indicator := ""
	if leadingSpaceBlock.MatchString(s) {
		indicator = strconv.Itoa(shiftOfContent - shiftOfParent)
	}
	clip := strings.HasSuffix(s, "\n")
	chomp := "-"
	if clip {
		chomp = ""
		if strings.HasSuffix(s, "\n\n") || s == "\n" {
			chomp = "+"
		}
	}
	return indicator + chomp + "\n"
}

func dropEndingNewline(s string) string {
	return strings.TrimSuffix(s, "\n")
}

func isMoreIndented(c uint16) bool { return c == ' ' || c == '\t' }

// foldLine ports js-yaml's foldLine on UTF-16 code units.
func foldLine(line []uint16, width int) []uint16 {
	if len(line) == 0 || isMoreIndented(line[0]) {
		return line
	}
	var result []uint16
	start, curr, next := 0, 0, 0
	for i := 0; i+1 < len(line); i++ {
		if line[i] != ' ' || line[i+1] == ' ' || line[i+1] == '\t' {
			continue
		}
		next = i
		if next-start > width {
			end := next
			if curr > start {
				end = curr
			}
			result = append(append(result, '\n'), line[start:end]...)
			start = end + 1
		}
		curr = next
	}
	result = append(result, '\n')
	if len(line)-start > width && curr > start {
		result = append(append(append(result, line[start:curr]...), '\n'), line[curr+1:]...)
	} else {
		result = append(result, line[start:]...)
	}
	return result[1:]
}

func foldBlockScalar(s string, width int) string {
	units := utf16.Encode([]rune(s))
	firstBreak := len(units)
	for i, c := range units {
		if c == '\n' {
			firstBreak = i
			break
		}
	}
	result := foldLine(units[:firstBreak], width)
	prevMoreIndented := len(units) > 0 && (units[0] == '\n' || isMoreIndented(units[0]))
	for i := firstBreak; i < len(units); {
		j := i
		for j < len(units) && units[j] == '\n' {
			j++
		}
		prefix := units[i:j]
		k := j
		for k < len(units) && units[k] != '\n' {
			k++
		}
		line := units[j:k]
		moreIndented := len(line) > 0 && isMoreIndented(line[0])
		result = append(result, prefix...)
		if !prevMoreIndented && !moreIndented && len(line) > 0 {
			result = append(result, '\n')
		}
		result = append(result, foldLine(line, width)...)
		prevMoreIndented = moreIndented
		i = k
	}
	return string(utf16.Decode(result))
}

func escapeYAMLString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r <= 0x1F || r >= 0x7F && r <= 0xA0 || r == 0x2028 || r == 0x2029 || r == 0xFEFF || r == 0xFFFE || r == 0xFFFF:
			b.WriteString(escapeYAMLRune(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeYAMLRune(r rune) string {
	switch r {
	case 0:
		return `\0`
	case 7:
		return `\a`
	case '\b':
		return `\b`
	case '\t':
		return `\t`
	case '\n':
		return `\n`
	case '\v':
		return `\v`
	case '\f':
		return `\f`
	case '\r':
		return `\r`
	case 0x1B:
		return `\e`
	case 0x85:
		return `\N`
	case 0xA0:
		return `\_`
	case 0x2028:
		return `\L`
	case 0x2029:
		return `\P`
	}
	hex := strings.ToUpper(strconv.FormatInt(int64(r), 16))
	if r <= 0xFF {
		return `\x` + strings.Repeat("0", 2-len(hex)) + hex
	}
	return `\u` + strings.Repeat("0", 4-len(hex)) + hex
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// isPrintable is c-printable.
func isPrintable(r rune) bool {
	return r == 0x09 || r == 0x0A || r == 0x0D || r >= 0x20 && r <= 0x7E || r == 0x85 ||
		r >= 0xA0 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF
}

// isNsChar is ns-char: printable, not a line break, BOM or white space.
func isNsChar(r rune) bool {
	return isPrintable(r) && r != '\n' && r != '\r' && r != 0xFEFF && r != ' ' && r != '\t'
}

func isIndicator(r rune) bool {
	return strings.ContainsRune("-?:,[]{}#&*!|>'\"%@`", r)
}

// plainScalar matches NS_PLAIN_FLOW_OUT (multiline) or NS_PLAIN_BLOCK_KEY
// (one line) by hand: RE2 has no lookahead.
func plainScalar(s string, multiline bool) bool {
	runes := []rune(s)
	nsAt := func(i int) bool { return i < len(runes) && isNsChar(runes[i]) }
	// plainChar consumes one ns-plain-char plus trailing '#'s.
	plainChar := func(i int) (int, bool) {
		if i >= len(runes) {
			return i, false
		}
		c := runes[i]
		switch {
		case c == ':' && nsAt(i+1):
			i++
		case c != ':' && c != '#' && isNsChar(c):
			i++
		default:
			return i, false
		}
		for i < len(runes) && runes[i] == '#' {
			i++
		}
		return i, true
	}
	// inLine consumes (s-white* plain-char)* up to a line break or the end.
	inLine := func(i int) (int, bool) {
		for {
			j := i
			for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
				j++
			}
			if j >= len(runes) || runes[j] == '\n' {
				return i, j == i
			}
			next, ok := plainChar(j)
			if !ok {
				return i, false
			}
			i = next
		}
	}
	if len(runes) == 0 {
		return false
	}
	first := runes[0]
	i := 0
	switch {
	case isNsChar(first) && !isIndicator(first):
		i = 1
	case (first == '?' || first == ':' || first == '-') && nsAt(1):
		i = 1
	default:
		return false
	}
	for i < len(runes) && runes[i] == '#' {
		i++
	}
	i, ok := inLine(i)
	if !ok {
		return false
	}
	for i < len(runes) {
		if !multiline || runes[i] != '\n' {
			return false
		}
		for i < len(runes) && runes[i] == '\n' {
			i++
		}
		if i, ok = plainChar(i); !ok {
			return false
		}
		if i, ok = inLine(i); !ok {
			return false
		}
	}
	return true
}

var (
	yaml11Null  = map[string]bool{"": true, "~": true, "null": true, "Null": true, "NULL": true}
	yaml11Bools = map[string]bool{
		"true": true, "True": true, "TRUE": true, "y": true, "Y": true, "yes": true, "Yes": true, "YES": true, "on": true, "On": true, "ON": true,
		"false": true, "False": true, "FALSE": true, "n": true, "N": true, "no": true, "No": true, "NO": true, "off": true, "Off": true, "OFF": true,
	}
	int11Pattern     = regexp.MustCompile(`^(?:[-+]?0b[0-1_]+|[-+]?0[0-7_]+|[-+]?0x[0-9a-fA-F_]+|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+|[-+]?(?:0|[1-9][0-9_]*))$`)
	intCorePattern   = regexp.MustCompile(`^(?:0o[0-7]+|0x[0-9a-fA-F]+|[-+]?[0-9]+)$`)
	float11Pattern   = regexp.MustCompile(`^(?:[-+]?(?:(?:[0-9][0-9_]*)?\.[0-9_]*)(?:[eE][-+][0-9]+)?|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
	floatCorePattern = regexp.MustCompile(`^(?:[-+]?[0-9]+(?:\.[0-9]*)?(?:[eE][-+]?[0-9]+)?|[-+]?\.[0-9]+(?:[eE][-+]?[0-9]+)?|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
	floatSpecial     = regexp.MustCompile(`^(?:[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
	jsFloatPrefix    = regexp.MustCompile(`^[+-]?(?:[0-9]+\.?[0-9]*(?:[eE][+-]?[0-9]+)?|\.[0-9]+(?:[eE][+-]?[0-9]+)?)`)
	dateOnly         = regexp.MustCompile(`^([0-9]{4})-([0-9]{2})-([0-9]{2})$`)
	timestampPattern = regexp.MustCompile(`^([0-9]{4})-([0-9][0-9]?)-([0-9][0-9]?)(?:[Tt]|[ \t]+)([0-9][0-9]?):([0-9][0-9]):([0-9][0-9])(?:\.([0-9]*))?(?:[ \t]*(Z|([-+])([0-9][0-9]?)(?::([0-9][0-9]))?))?$`)
)

// jsParseFloat is parseFloat: the longest numeric prefix, NaN without one.
func jsParseFloat(s string) float64 {
	prefix := jsFloatPrefix.FindString(s)
	if prefix == "" {
		return math.NaN()
	}
	value, err := strconv.ParseFloat(prefix, 64)
	if err != nil && !math.IsInf(value, 0) {
		return math.NaN()
	}
	return value
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func resolvesInt(s string) bool {
	if intCorePattern.MatchString(s) {
		digits := strings.TrimLeft(s, "+-")
		if strings.HasPrefix(digits, "0o") || strings.HasPrefix(digits, "0x") {
			return true
		}
		return finite(jsParseFloat(digits))
	}
	if !int11Pattern.MatchString(s) {
		return false
	}
	value := strings.TrimLeft(strings.ReplaceAll(s, "_", ""), "+-")
	switch {
	case strings.HasPrefix(value, "0b"), strings.HasPrefix(value, "0x"):
		return len(value) > 2
	case strings.Contains(value, ":"):
		return true
	}
	return finite(jsParseFloat(value))
}

func resolvesFloat(s string) bool {
	if float11Pattern.MatchString(s) {
		value := strings.TrimLeft(strings.ReplaceAll(strings.ToLower(s), "_", ""), "+-")
		if value == ".inf" || value == ".nan" || strings.Contains(value, ":") {
			return true
		}
		if finite(jsParseFloat(value)) || floatSpecial.MatchString(s) {
			return true
		}
	}
	if floatCorePattern.MatchString(s) {
		value := strings.TrimLeft(strings.ToLower(s), "+-")
		if value == ".inf" || value == ".nan" {
			return true
		}
		return finite(jsParseFloat(value)) || floatSpecial.MatchString(s)
	}
	return false
}

func resolvesTimestamp(s string) bool {
	match := dateOnly.FindStringSubmatch(s)
	if match == nil {
		match = timestampPattern.FindStringSubmatch(s)
	}
	if match == nil {
		return false
	}
	atoi := func(v string) int { n, _ := strconv.Atoi(v); return n }
	year, month, day := atoi(match[1]), atoi(match[2])-1, atoi(match[3])
	valid := func(hour, minute, second int) bool {
		date := time.Date(year, time.Month(month+1), day, hour, minute, second, 0, time.UTC)
		return date.Year() == year && int(date.Month())-1 == month && date.Day() == day
	}
	if len(match) == 4 || match[4] == "" {
		return valid(0, 0, 0)
	}
	hour, minute, second := atoi(match[4]), atoi(match[5]), atoi(match[6])
	if hour > 23 || minute > 59 || second > 59 || !valid(hour, minute, second) {
		return false
	}
	if match[9] != "" && (atoi(match[10]) > 23 || atoi(match[11]) > 59) {
		return false
	}
	return true
}

// implicitTag is DUMP_SCHEMA.resolveImplicitScalarTag reduced to the tag
// name.
func implicitTag(s string) string {
	switch {
	case yaml11Null[s]:
		return "null"
	case yaml11Bools[s]:
		return "bool"
	case s == "":
		return "str"
	}
	switch first := s[0]; {
	case first == '-' || first == '+' || first >= '0' && first <= '9':
		if resolvesInt(s) {
			return "int"
		}
		if resolvesFloat(s) {
			return "float"
		}
		if first >= '0' && first <= '9' && resolvesTimestamp(s) {
			return "timestamp"
		}
	case first == '.':
		if resolvesFloat(s) {
			return "float"
		}
	case first == '<':
		if s == "<<" {
			return "merge"
		}
	}
	return "str"
}
