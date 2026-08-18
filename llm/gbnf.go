package llm

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
)

// jsonSchemaToGBNF converts a JSON schema (as decoded from JSON, i.e.
// map[string]any / []any trees) into a GBNF grammar string usable with
// llama.cpp's grammar sampler (llama.SamplerInitGrammar).
//
// It mirrors llama.cpp's common/json-schema-to-grammar.cpp for the supported
// subset: const, enum, type (including arrays of types), objects with
// properties/required/additionalProperties, arrays with items/prefixItems/
// minItems/maxItems, oneOf/anyOf/allOf, local #/... $refs, string
// minLength/maxLength, integer minimum/maximum/exclusive bounds, and the
// date/time/date-time/uuid formats.
//
// Known limitations: JSON Schema regex `pattern` keywords are not converted
// (the schema degrades to an unconstrained string, with a warning) and remote
// (non-local) $refs are rejected.
func jsonSchemaToGBNF(schema any) (string, error) {
	b := newGBNFBuilder(schema)
	rootRule, err := b.visit(schema)
	if err != nil {
		return "", err
	}
	lines := append([]string{"root ::= " + rootRule}, b.rules...)
	return strings.Join(lines, "\n") + "\n", nil
}

// grammarFromResponseFormat extracts the JSON schema from an OpenAI-style
// response_format value ({type:"json_schema", json_schema:{schema: ...}})
// and converts it to a GBNF grammar.
func grammarFromResponseFormat(responseFormat any) (string, error) {
	schema := schemaObjectFromResponseFormat(responseFormat)
	if schema == nil {
		return "", fmt.Errorf("structured output requires a json_schema response_format")
	}
	return jsonSchemaToGBNF(schema)
}

// schemaObjectFromResponseFormat returns the raw schema object nested inside
// an OpenAI-style response_format value, or nil when the shape does not match.
func schemaObjectFromResponseFormat(responseFormat any) any {
	rf, ok := responseFormat.(map[string]any)
	if !ok {
		return nil
	}
	js, ok := rf["json_schema"].(map[string]any)
	if !ok {
		return nil
	}
	return js["schema"]
}

// gbnfPrimitives are the shared scalar/structural rules, mirroring llama.cpp's
// PRIMITIVE_RULES. The names are reserved rule identifiers (rule references
// generated for unconstrained subschemas point at them).
var gbnfPrimitives = map[string]string{
	"space":         `| " " | "\n"{1,2} [ \t]{0,20}`,
	"boolean":       `("true" | "false")`,
	"decimal-part":  `[0-9]{1,16}`,
	"integral-part": `[0] | [1-9] [0-9]{0,15}`,
	"number":        `("-"? integral-part) ("." decimal-part)? ([eE] [-+]? integral-part)?`,
	"integer":       `("-"? integral-part)`,
	"value":         `object | array | string | number | boolean | null`,
	"object":        `"{" space ( string ":" space value ("," space string ":" space value)* )? space "}"`,
	"array":         `"[" space ( value ("," space value)* )? space "]"`,
	"uuid":          `"\"" [0-9a-fA-F]{8} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{12} "\""`,
	"char":          `[^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})`,
	"string":        `"\"" char* "\""`,
	"null":          `"null"`,
}

// gbnfPrimitiveDeps declares which primitive rules each primitive rule
// references, so they can be emitted on demand.
var gbnfPrimitiveDeps = map[string][]string{
	"number":  {"integral-part", "decimal-part"},
	"integer": {"integral-part"},
	"value":   {"object", "array", "string", "number", "boolean", "null"},
	"object":  {"string", "value"},
	"array":   {"value"},
	"string":  {"char"},
}

// gbnfStringFormats are the date/time/date-time formats (without surrounding
// quotes); date-time references the date and time rules.
var gbnfStringFormats = map[string]string{
	"date":      `[0-9]{4} "-" ( "0" [1-9] | "1" [0-2] ) "-" ( "0" [1-9] | [1-2] [0-9] | "3" [0-1] )`,
	"time":      `([01] [0-9] | "2" [0-3]) ":" [0-5] [0-9] ":" [0-5] [0-9] ( "." [0-9]{3} )? ( "Z" | ( "+" | "-" ) ( [01] [0-9] | "2" [0-3] ) ":" [0-5] [0-9] )`,
	"date-time": `date "T" time`,
}

var gbnfStringFormatDeps = map[string][]string{
	"date-time": {"date", "time"},
}

type gbnfBuilder struct {
	rules     []string        // accumulated "name ::= def" lines
	names     map[string]bool // rule names already registered
	counter   int
	refs      map[string]any  // resolved local #/... $ref pointers
	resolving map[string]bool // refs currently being resolved (cycle guard)
}

func newGBNFBuilder(root any) *gbnfBuilder {
	b := &gbnfBuilder{
		rules:     []string{`space ::= | " " | "\n"{1,2} [ \t]{0,20}`},
		names:     map[string]bool{"space": true},
		refs:      map[string]any{},
		resolving: map[string]bool{},
	}
	collectRefs(root, b.refs, root)
	return b
}

func (b *gbnfBuilder) addRule(name, def string) string {
	if !b.names[name] {
		b.names[name] = true
		b.rules = append(b.rules, name+" ::= "+def)
	}
	return name
}

func (b *gbnfBuilder) freshName(prefix string) string {
	for {
		b.counter++
		name := fmt.Sprintf("%s-%d", prefix, b.counter)
		if !b.names[name] {
			return name
		}
	}
}

// primitiveRef registers (once) the named primitive rule and its dependencies
// and returns the rule name to reference it.
func (b *gbnfBuilder) primitiveRef(name string) (string, error) {
	def, ok := gbnfPrimitives[name]
	if !ok {
		return "", fmt.Errorf("unknown primitive rule %q", name)
	}
	if !b.names[name] {
		// Register the rule before its dependencies so that cyclic
		// dependencies (e.g. value -> object -> value) terminate.
		b.addRule(name, def)
		for _, dep := range gbnfPrimitiveDeps[name] {
			if _, err := b.primitiveRef(dep); err != nil {
				return "", err
			}
		}
	}
	return name, nil
}

// literalRule registers a rule whose body is a single inline definition.
func (b *gbnfBuilder) literalRule(content string) string {
	name := b.freshName("lit")
	b.addRule(name, content)
	return name
}

// unionSchemas registers a rule matching any of the given subschemas.
func (b *gbnfBuilder) unionSchemas(alts []any) (string, error) {
	if len(alts) == 0 {
		return "", fmt.Errorf("oneOf/anyOf must be a non-empty array")
	}
	names := make([]string, 0, len(alts))
	for _, sub := range alts {
		r, err := b.visit(sub)
		if err != nil {
			return "", err
		}
		names = append(names, r)
	}
	name := b.freshName("union")
	b.addRule(name, strings.Join(names, " | "))
	return name, nil
}

// visit converts one schema node into a GBNF rule name (either a registered
// primitive or a freshly generated rule). The dispatch order mirrors
// llama.cpp's common_schema_converter::visit.
func (b *gbnfBuilder) visit(schema any) (string, error) {
	m, ok := schema.(map[string]any)
	if !ok {
		return "", fmt.Errorf("JSON schema node must be an object")
	}

	// $ref (local #/ pointers only)
	if r, ok := m["$ref"].(string); ok {
		return b.resolveRef(r)
	}

	// oneOf/anyOf
	if u, ok := m["oneOf"]; ok {
		arr, ok := u.([]any)
		if !ok {
			return "", fmt.Errorf("oneOf must be an array")
		}
		return b.unionSchemas(arr)
	}
	if u, ok := m["anyOf"]; ok {
		arr, ok := u.([]any)
		if !ok {
			return "", fmt.Errorf("anyOf must be an array")
		}
		return b.unionSchemas(arr)
	}

	schemaType, _ := m["type"].(string)

	// type as an array -> union of each individual type
	if ta, ok := m["type"].([]any); ok && len(ta) > 0 {
		alts := make([]any, 0, len(ta))
		for _, t := range ta {
			ts, ok := t.(string)
			if !ok {
				return "", fmt.Errorf("type array entries must be strings")
			}
			clone := make(map[string]any, len(m))
			for k, v := range m {
				clone[k] = v
			}
			clone["type"] = ts
			alts = append(alts, clone)
		}
		return b.unionSchemas(alts)
	}

	// const
	if c, ok := m["const"]; ok {
		return b.literalRule(gbnfFormatLiteral(jsonDump(c))), nil
	}

	// enum
	if e, ok := m["enum"]; ok {
		if arr, ok := e.([]any); ok && len(arr) > 0 {
			vals := make([]string, 0, len(arr))
			for _, v := range arr {
				vals = append(vals, gbnfFormatLiteral(jsonDump(v)))
			}
			return b.literalRule("(" + strings.Join(vals, " | ") + ")"), nil
		}
	}

	// object with properties / additionalProperties
	if (schemaType == "" || schemaType == "object") &&
		(m["properties"] != nil || (m["additionalProperties"] != nil && m["additionalProperties"] != true)) {
		return b.buildObjectRule(m)
	}

	// allOf
	if (schemaType == "" || schemaType == "object" || schemaType == "string") && m["allOf"] != nil {
		return b.buildAllOf(m)
	}

	// array
	if (schemaType == "" || schemaType == "array") && (m["items"] != nil || m["prefixItems"] != nil) {
		return b.buildArrayRule(m)
	}

	// pattern (unsupported: degrade to an unconstrained string)
	if (schemaType == "" || schemaType == "string") && m["pattern"] != nil {
		slog.Warn("json schema pattern not supported, degrading to unconstrained string")
	}

	format, _ := m["format"].(string)

	// uuid format
	if (schemaType == "" || schemaType == "string") && isUUIDFormat(format) {
		return b.primitiveRef("uuid")
	}

	// date/time/date-time formats
	if schemaType == "" || schemaType == "string" {
		if _, isFmt := gbnfStringFormats[format]; isFmt {
			return b.stringFormatRule(format)
		}
	}

	// string with minLength/maxLength
	if schemaType == "string" && (m["minLength"] != nil || m["maxLength"] != nil) {
		return b.buildStringLength(m)
	}

	// integer with numeric bounds
	if schemaType == "integer" &&
		(m["minimum"] != nil || m["exclusiveMinimum"] != nil || m["maximum"] != nil || m["exclusiveMaximum"] != nil) {
		return b.buildIntegerBounds(m)
	}

	// empty schema or explicit object type -> generic object
	if len(m) == 0 || schemaType == "object" {
		return b.primitiveRef("object")
	}

	// no type and no recognized structural keywords -> any value
	if schemaType == "" {
		return b.primitiveRef("value")
	}

	// unknown type
	if _, ok := gbnfPrimitives[schemaType]; !ok {
		return "", fmt.Errorf("unsupported JSON schema type %q", schemaType)
	}

	return b.primitiveRef(schemaType)
}

func (b *gbnfBuilder) resolveRef(ref string) (string, error) {
	if !strings.HasPrefix(ref, "#/") {
		return "", fmt.Errorf("unsupported $ref %q (only local #/ pointers are supported)", ref)
	}
	if b.resolving[ref] {
		return b.primitiveRef("value")
	}
	target, ok := b.refs[ref]
	if !ok {
		return "", fmt.Errorf("cannot resolve $ref %q", ref)
	}
	b.resolving[ref] = true
	name, err := b.visit(target)
	delete(b.resolving, ref)
	return name, err
}

func (b *gbnfBuilder) buildObjectRule(m map[string]any) (string, error) {
	props, _ := m["properties"].(map[string]any)
	requiredSet := map[string]bool{}
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	// Go maps have no order; sort property names for deterministic output.
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)

	var requiredProps, optionalProps []string
	propKV := map[string]string{}
	for _, p := range names {
		vr, err := b.visit(props[p])
		if err != nil {
			return "", err
		}
		kv := b.freshName("prop")
		b.addRule(kv, gbnfStringLiteral(p)+" space \":\" space "+vr)
		propKV[p] = kv
		if requiredSet[p] {
			requiredProps = append(requiredProps, p)
		} else {
			optionalProps = append(optionalProps, p)
		}
	}

	// additionalProperties (true or a schema)
	if additional, has := m["additionalProperties"]; has {
		apBool, isBool := additional.(bool)
		if (isBool && apBool) || (!isBool && additional != nil) {
			var valueRule string
			var err error
			if !isBool {
				valueRule, err = b.visit(additional)
			} else {
				valueRule, err = b.primitiveRef("value")
			}
			if err != nil {
				return "", err
			}
			var keyRule string
			if len(names) == 0 {
				keyRule, err = b.primitiveRef("string")
			} else {
				keyRule = b.notStrings(names)
			}
			if err != nil {
				return "", err
			}
			kv := b.freshName("add")
			b.addRule(kv, keyRule+" space \":\" space "+valueRule)
			propKV["*"] = kv
			optionalProps = append(optionalProps, "*")
		}
	}

	var rule strings.Builder
	rule.WriteString(`"{" space `)
	for i, p := range requiredProps {
		if i > 0 {
			rule.WriteString(` "," space `)
		}
		rule.WriteString(propKV[p])
	}
	if len(optionalProps) > 0 {
		rule.WriteString(" (")
		if len(requiredProps) > 0 {
			rule.WriteString(` "," space ( `)
		}
		for i := 0; i < len(optionalProps); i++ {
			if i > 0 {
				rule.WriteString(" | ")
			}
			rule.WriteString(b.optionalRefs(optionalProps[i:], propKV, false))
		}
		if len(requiredProps) > 0 {
			rule.WriteString(" )")
		}
		rule.WriteString(" )?")
	}
	rule.WriteString(` space "}"`)

	name := b.freshName("object")
	b.addRule(name, rule.String())
	return name, nil
}

// optionalRefs renders one alternative of the optional/additional member
// sequence for a suffix of the optional property list, mirroring llama.cpp's
// get_recursive_refs. Each optional member may be omitted; the additional "*"
// member may repeat. Members always appear in sorted order.
func (b *gbnfBuilder) optionalRefs(ks []string, propKV map[string]string, firstIsOptional bool) string {
	if len(ks) == 0 {
		return ""
	}
	k := ks[0]
	kv := propKV[k]
	commaRef := "( \",\" space " + kv + " )"
	var res string
	if firstIsOptional {
		if k == "*" {
			res = commaRef + "*"
		} else {
			res = commaRef + "?"
		}
	} else {
		if k == "*" {
			res = kv + " " + commaRef + "*"
		} else {
			res = kv
		}
	}
	if len(ks) > 1 {
		rest := b.optionalRefs(ks[1:], propKV, true)
		restName := b.freshName("rest")
		b.addRule(restName, rest)
		res += " " + restName
	}
	return res
}

// notStrings builds a rule that matches any JSON string except the given
// property names (used for the key of additional properties so known keys
// cannot be emitted twice). Mirrors llama.cpp's _not_strings.
func (b *gbnfBuilder) notStrings(strs []string) string {
	type trieNode struct {
		children map[byte]*trieNode
		end      bool
	}
	root := &trieNode{children: map[byte]*trieNode{}}
	for _, s := range strs {
		node := root
		for i := 0; i < len(s); i++ {
			c := s[i]
			if node.children[c] == nil {
				node.children[c] = &trieNode{children: map[byte]*trieNode{}}
			}
			node = node.children[c]
		}
		node.end = true
	}
	charRule, _ := b.primitiveRef("char")

	var out strings.Builder
	out.WriteString(`["] (`)
	var visit func(n *trieNode)
	visit = func(n *trieNode) {
		var rejects strings.Builder
		first := true
		children := make([]byte, 0, len(n.children))
		for c := range n.children {
			children = append(children, c)
		}
		sort.Slice(children, func(i, j int) bool { return children[i] < children[j] })
		for _, c := range children {
			child := n.children[c]
			rejects.WriteByte(c)
			if first {
				first = false
			} else {
				out.WriteString(" | ")
			}
			out.WriteString("[" + string(c) + "]")
			if len(child.children) > 0 {
				out.WriteString(" (")
				visit(child)
				out.WriteString(")")
			} else if child.end {
				out.WriteString(" " + charRule + "+")
			}
		}
		if len(n.children) > 0 {
			if !first {
				out.WriteString(" | ")
			}
			out.WriteString(`[^"` + rejects.String() + `] ` + charRule + `*`)
		}
	}
	visit(root)
	out.WriteString(" )")
	if !root.end {
		out.WriteString("?")
	}
	out.WriteString(` ["]`)

	name := b.freshName("addk")
	b.addRule(name, out.String())
	return name
}

// buildAllOf merges the allOf components: object properties/required are
// unioned, and the intersection of enum values (when every component lists
// one) is emitted. Mirrors llama.cpp's allOf handling.
func (b *gbnfBuilder) buildAllOf(m map[string]any) (string, error) {
	arr, ok := m["allOf"].([]any)
	if !ok {
		return "", fmt.Errorf("allOf must be an array")
	}
	required := map[string]bool{}
	props := map[string]any{}
	enumCounts := map[string]int{}

	var addComponent func(comp map[string]any, isRequired bool) error
	addComponent = func(comp map[string]any, isRequired bool) error {
		if r, ok := comp["$ref"].(string); ok {
			if !strings.HasPrefix(r, "#/") {
				return fmt.Errorf("unsupported $ref %q inside allOf", r)
			}
			target, ok := b.refs[r]
			if !ok {
				return fmt.Errorf("cannot resolve $ref %q inside allOf", r)
			}
			tm, ok := target.(map[string]any)
			if !ok {
				return fmt.Errorf("$ref %q inside allOf must point to an object", r)
			}
			return addComponent(tm, isRequired)
		}
		if p, ok := comp["properties"].(map[string]any); ok {
			for k, v := range p {
				props[k] = v
				if isRequired {
					required[k] = true
				}
			}
		}
		if e, ok := comp["enum"].([]any); ok {
			for _, v := range e {
				enumCounts[jsonDump(v)]++
			}
		}
		return nil
	}

	for _, t := range arr {
		tm, ok := t.(map[string]any)
		if !ok {
			return "", fmt.Errorf("allOf entries must be objects")
		}
		if u, ok := tm["anyOf"].([]any); ok {
			for _, tt := range u {
				ttm, ok := tt.(map[string]any)
				if !ok {
					return "", fmt.Errorf("anyOf entries inside allOf must be objects")
				}
				if err := addComponent(ttm, false); err != nil {
					return "", err
				}
			}
		} else if err := addComponent(tm, true); err != nil {
			return "", err
		}
	}

	if len(enumCounts) > 0 {
		var inter []string
		for v, c := range enumCounts {
			if c == len(arr) {
				inter = append(inter, gbnfFormatLiteral(v))
			}
		}
		if len(inter) > 0 {
			sort.Strings(inter)
			return b.literalRule("(" + strings.Join(inter, " | ") + ")"), nil
		}
	}

	merged := map[string]any{}
	if len(props) > 0 {
		merged["properties"] = props
	}
	if len(required) > 0 {
		var req []any
		for k := range required {
			req = append(req, k)
		}
		sort.Slice(req, func(i, j int) bool { return req[i].(string) < req[j].(string) })
		merged["required"] = req
	}
	return b.buildObjectRule(merged)
}

func (b *gbnfBuilder) buildArrayRule(m map[string]any) (string, error) {
	items, hasItems := m["items"]
	if !hasItems {
		items = m["prefixItems"]
	}

	// tuple array: items is an array of per-position schemas
	if arr, ok := items.([]any); ok {
		var rule strings.Builder
		rule.WriteString(`"[" space `)
		for i, item := range arr {
			if i > 0 {
				rule.WriteString(` "," space `)
			}
			r, err := b.visit(item)
			if err != nil {
				return "", err
			}
			rule.WriteString(r)
		}
		rule.WriteString(` space "]"`)
		name := b.freshName("array")
		b.addRule(name, rule.String())
		return name, nil
	}

	itemRule, err := b.visit(items)
	if err != nil {
		return "", err
	}
	minItems := 0
	if v, ok := m["minItems"].(float64); ok {
		minItems = int(v)
	}
	maxItems := math.MaxInt
	if v, ok := m["maxItems"].(float64); ok {
		maxItems = int(v)
	}
	if minItems < 0 {
		minItems = 0
	}
	if maxItems < minItems {
		maxItems = minItems
	}
	rule := `"[" space ` + b.buildRepetition(itemRule, minItems, maxItems, `"," space`) + ` space "]"`
	name := b.freshName("array")
	b.addRule(name, rule)
	return name, nil
}

func (b *gbnfBuilder) buildStringLength(m map[string]any) (string, error) {
	cr, err := b.primitiveRef("char")
	if err != nil {
		return "", err
	}
	minLen := 0
	if v, ok := m["minLength"].(float64); ok {
		minLen = int(v)
	}
	maxLen := math.MaxInt
	if v, ok := m["maxLength"].(float64); ok {
		maxLen = int(v)
	}
	if minLen < 0 {
		minLen = 0
	}
	if maxLen < minLen {
		maxLen = minLen
	}
	name := b.freshName("str")
	b.addRule(name, `"\"" `+b.buildRepetition(cr, minLen, maxLen, "")+` "\""`)
	return name, nil
}

func (b *gbnfBuilder) buildIntegerBounds(m map[string]any) (string, error) {
	minValue, maxValue := int64(math.MinInt64), int64(math.MaxInt64)
	if v, ok := m["minimum"].(float64); ok {
		minValue = int64(v)
	} else if v, ok := m["exclusiveMinimum"].(float64); ok {
		minValue = int64(v) + 1
	}
	if v, ok := m["maximum"].(float64); ok {
		maxValue = int64(v)
	} else if v, ok := m["exclusiveMaximum"].(float64); ok {
		maxValue = int64(v) - 1
	}
	if minValue > maxValue {
		return "", fmt.Errorf("invalid integer bounds: min %d > max %d", minValue, maxValue)
	}
	var out strings.Builder
	out.WriteByte('(')
	b.buildMinMaxInt(minValue, maxValue, &out, 16, true)
	out.WriteByte(')')
	name := b.freshName("int")
	b.addRule(name, out.String())
	return name, nil
}

func (b *gbnfBuilder) buildRepetition(itemRule string, minItems, maxItems int, sep string) string {
	hasMax := maxItems != math.MaxInt
	if maxItems == 0 {
		return ""
	}
	if minItems == 0 && maxItems == 1 {
		return itemRule + "?"
	}
	if sep == "" {
		switch {
		case minItems == 1 && !hasMax:
			return itemRule + "+"
		case minItems == 0 && !hasMax:
			return itemRule + "*"
		default:
			mx := ""
			if hasMax {
				mx = fmt.Sprintf("%d", maxItems)
			}
			return fmt.Sprintf("%s{%d,%s}", itemRule, minItems, mx)
		}
	}
	subMin := minItems - 1
	if subMin < 0 {
		subMin = 0
	}
	subMax := maxItems
	if hasMax {
		subMax = maxItems - 1
	}
	result := itemRule + " " + b.buildRepetition("("+sep+" "+itemRule+")", subMin, subMax, "")
	if minItems == 0 {
		result = "(" + result + ")?"
	}
	return result
}

// buildMinMaxInt writes a GBNF expression matching exactly the integers in
// [minValue, maxValue]. Port of llama.cpp's build_min_max_int (see
// common/json-schema-to-grammar.cpp). At least one bound must be finite.
func (b *gbnfBuilder) buildMinMaxInt(minValue, maxValue int64, out *strings.Builder, decimalsLeft int, topLevel bool) {
	hasMin := minValue != math.MinInt64
	hasMax := maxValue != math.MaxInt64

	digitRange := func(from, to byte) {
		out.WriteByte('[')
		if from == to {
			out.WriteByte(from)
		} else {
			out.WriteByte(from)
			out.WriteByte('-')
			out.WriteByte(to)
		}
		out.WriteByte(']')
	}
	moreDigits := func(minDigits, maxDigits int) {
		out.WriteString("[0-9]")
		if minDigits == maxDigits && minDigits == 1 {
			return
		}
		out.WriteByte('{')
		out.WriteString(strconv.Itoa(minDigits))
		if maxDigits != minDigits {
			out.WriteByte(',')
			if maxDigits != math.MaxInt {
				out.WriteString(strconv.Itoa(maxDigits))
			}
		}
		out.WriteByte('}')
	}

	// uniformRange writes a GBNF expression matching every number between two
	// equal-length digit strings (lexicographic order == numeric order).
	var uniformRange func(from, to string)
	uniformRange = func(from, to string) {
		i := 0
		for i < len(from) && i < len(to) && from[i] == to[i] {
			i++
		}
		if i > 0 {
			out.WriteString(`"` + from[:i] + `"`)
		}
		if i < len(from) && i < len(to) {
			if i > 0 {
				out.WriteByte(' ')
			}
			subLen := len(from) - i - 1
			if subLen > 0 {
				fromSub := from[i+1:]
				toSub := to[i+1:]
				subZeros := strings.Repeat("0", subLen)
				subNines := strings.Repeat("9", subLen)
				toReached := false
				out.WriteString("(")
				if fromSub == subZeros {
					digitRange(from[i], to[i]-1)
					out.WriteByte(' ')
					moreDigits(subLen, subLen)
				} else {
					out.WriteString("[" + string(from[i]) + "] ")
					out.WriteString("(")
					uniformRange(fromSub, subNines)
					out.WriteString(")")
					if from[i] < to[i]-1 {
						out.WriteString(" | ")
						if toSub == subNines {
							digitRange(from[i]+1, to[i])
							toReached = true
						} else {
							digitRange(from[i]+1, to[i]-1)
						}
						out.WriteByte(' ')
						moreDigits(subLen, subLen)
					}
				}
				if !toReached {
					out.WriteString(" | ")
					digitRange(to[i], to[i])
					out.WriteByte(' ')
					uniformRange(subZeros, toSub)
				}
				out.WriteString(")")
			} else {
				out.WriteString("[" + string(from[i]) + "-" + string(to[i]) + "]")
			}
		}
	}

	if hasMin && hasMax {
		if minValue < 0 && maxValue < 0 {
			out.WriteString(`"-" (`)
			b.buildMinMaxInt(-maxValue, -minValue, out, decimalsLeft, true)
			out.WriteString(")")
			return
		}
		if minValue < 0 {
			out.WriteString(`"-" (`)
			b.buildMinMaxInt(0, -minValue, out, decimalsLeft, true)
			out.WriteString(") | ")
			minValue = 0
		}
		minS := strconv.FormatInt(minValue, 10)
		maxS := strconv.FormatInt(maxValue, 10)
		for digits := len(minS); digits < len(maxS); digits++ {
			uniformRange(minS, strings.Repeat("9", digits))
			minS = "1" + strings.Repeat("0", digits)
			out.WriteString(" | ")
		}
		uniformRange(minS, maxS)
		return
	}

	lessDecimals := decimalsLeft - 1
	if lessDecimals < 1 {
		lessDecimals = 1
	}

	if hasMin {
		switch {
		case minValue < 0:
			out.WriteString(`"-" (`)
			b.buildMinMaxInt(math.MinInt64, -minValue, out, decimalsLeft, false)
			out.WriteString(") | [0] | [1-9] ")
			moreDigits(0, decimalsLeft-1)
		case minValue == 0:
			if topLevel {
				out.WriteString("[0] | [1-9] ")
				moreDigits(0, lessDecimals)
			} else {
				moreDigits(1, decimalsLeft)
			}
		case minValue <= 9:
			c := byte('0' + minValue)
			rangeStart := byte('1')
			if !topLevel {
				rangeStart = '0'
			}
			if c > rangeStart {
				digitRange(rangeStart, c-1)
				out.WriteByte(' ')
				moreDigits(1, lessDecimals)
				out.WriteString(" | ")
			}
			digitRange(c, '9')
			out.WriteByte(' ')
			moreDigits(0, lessDecimals)
		default:
			minS := strconv.FormatInt(minValue, 10)
			length := len(minS)
			c := minS[0]
			if c > '1' {
				rangeStart := byte('1')
				if !topLevel {
					rangeStart = '0'
				}
				digitRange(rangeStart, c-1)
				out.WriteByte(' ')
				moreDigits(length, lessDecimals)
				out.WriteString(" | ")
			}
			digitRange(c, c)
			out.WriteString(" (")
			sub, _ := strconv.ParseInt(minS[1:], 10, 64)
			b.buildMinMaxInt(sub, math.MaxInt64, out, lessDecimals, false)
			out.WriteString(")")
			if c < '9' {
				out.WriteString(" | ")
				digitRange(c+1, '9')
				out.WriteByte(' ')
				moreDigits(length-1, lessDecimals)
			}
		}
		return
	}

	if hasMax {
		if maxValue >= 0 {
			if topLevel {
				out.WriteString(`"-" [1-9] `)
				moreDigits(0, lessDecimals)
				out.WriteString(" | ")
			}
			b.buildMinMaxInt(0, maxValue, out, decimalsLeft, true)
		} else {
			out.WriteString(`"-" (`)
			b.buildMinMaxInt(-maxValue, math.MaxInt64, out, decimalsLeft, false)
			out.WriteString(")")
		}
		return
	}

	// Unreachable in practice: at least one bound is always set by callers.
	out.WriteString("([0] | [1-9] [0-9]{0,15})")
}

func (b *gbnfBuilder) stringFormatRule(format string) (string, error) {
	if _, ok := gbnfStringFormats[format]; !ok {
		return "", fmt.Errorf("unknown string format %q", format)
	}
	if err := b.ensureFormatRule(format); err != nil {
		return "", err
	}
	name := b.freshName("fmt")
	b.addRule(name, `"\"" `+format+` "\""`)
	return name, nil
}

func (b *gbnfBuilder) ensureFormatRule(name string) error {
	if b.names[name] {
		return nil
	}
	for _, dep := range gbnfStringFormatDeps[name] {
		if err := b.ensureFormatRule(dep); err != nil {
			return err
		}
	}
	def, ok := gbnfStringFormats[name]
	if !ok {
		return fmt.Errorf("unknown string format %q", name)
	}
	b.addRule(name, def)
	return nil
}

func isUUIDFormat(f string) bool {
	if f == "uuid" {
		return true
	}
	if len(f) == 5 && strings.HasPrefix(f, "uuid") && f[4] >= '1' && f[4] <= '5' {
		return true
	}
	return false
}

// collectRefs walks the schema and records every local "#/..." $ref target.
func collectRefs(schema any, refs map[string]any, root any) {
	switch v := schema.(type) {
	case map[string]any:
		if r, ok := v["$ref"].(string); ok && strings.HasPrefix(r, "#/") {
			if _, seen := refs[r]; !seen {
				if target, ok := resolveJSONPointer(root, r[1:]); ok {
					refs[r] = target
					collectRefs(target, refs, root)
				}
			}
		}
		for _, val := range v {
			collectRefs(val, refs, root)
		}
	case []any:
		for _, item := range v {
			collectRefs(item, refs, root)
		}
	}
}

// resolveJSONPointer walks a JSON Pointer (without the leading '#') from root.
func resolveJSONPointer(root any, pointer string) (any, bool) {
	cur := root
	for _, p := range strings.Split(pointer, "/") {
		if p == "" {
			continue
		}
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// gbnfFormatLiteral wraps a string as a GBNF string literal, escaping the
// characters that are special inside one.
func gbnfFormatLiteral(s string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\r':
			out.WriteString(`\r`)
		case '\n':
			out.WriteString(`\n`)
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// gbnfStringLiteral renders a property name as a GBNF literal matching the
// JSON-encoded form of the key (i.e. including the surrounding quotes), the
// same way llama.cpp does with format_literal(json(key).dump()).
func gbnfStringLiteral(key string) string {
	b, _ := json.Marshal(key)
	return gbnfFormatLiteral(string(b))
}

func jsonDump(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
