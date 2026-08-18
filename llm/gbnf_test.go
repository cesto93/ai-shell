package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func gbnfFor(t *testing.T, schemaJSON string) string {
	t.Helper()
	var schema any
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		t.Fatalf("invalid schema JSON: %v", err)
	}
	grammar, err := jsonSchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("jsonSchemaToGBNF: %v", err)
	}
	return grammar
}

func gbnfRules(grammar string) map[string]string {
	rules := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(grammar), "\n") {
		if i := strings.Index(line, " ::= "); i > 0 {
			rules[line[:i]] = line[i+5:]
		}
	}
	return rules
}

func TestJSONSchemaToGBNF(t *testing.T) {
	cases := []struct {
		name   string
		schema string
		want   string
	}{
		{
			"simple",
			`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"],"additionalProperties":false}`,
			`root ::= object-3
space ::= | " " | "\n"{1,2} [ \t]{0,20}
integer ::= ("-"? integral-part)
integral-part ::= [0] | [1-9] [0-9]{0,15}
prop-1 ::= "\"age\"" space ":" space integer
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
prop-2 ::= "\"name\"" space ":" space string
object-3 ::= "{" space prop-2 ( "," space ( prop-1 ) )? space "}"
`,
		},
		{
			"enum",
			`{"type":"string","enum":["red","green","blue"]}`,
			`root ::= lit-1
space ::= | " " | "\n"{1,2} [ \t]{0,20}
lit-1 ::= ("\"red\"" | "\"green\"" | "\"blue\"")
`,
		},
		{
			"array-minItems",
			`{"type":"array","items":{"type":"boolean"},"minItems":2}`,
			`root ::= array-1
space ::= | " " | "\n"{1,2} [ \t]{0,20}
boolean ::= ("true" | "false")
array-1 ::= "[" space boolean ("," space boolean)+ space "]"
`,
		},
		{
			"anyof",
			`{"anyOf":[{"type":"string"},{"type":"integer"}]}`,
			`root ::= union-1
space ::= | " " | "\n"{1,2} [ \t]{0,20}
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
integer ::= ("-"? integral-part)
integral-part ::= [0] | [1-9] [0-9]{0,15}
union-1 ::= string | integer
`,
		},
		{
			"strlen",
			`{"type":"string","minLength":1,"maxLength":4}`,
			`root ::= str-1
space ::= | " " | "\n"{1,2} [ \t]{0,20}
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
str-1 ::= "\"" char{1,4} "\""
`,
		},
		{
			"empty",
			`{}`,
			`root ::= object
space ::= | " " | "\n"{1,2} [ \t]{0,20}
object ::= "{" space ( string ":" space value ("," space string ":" space value)* )? space "}"
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
value ::= object | array | string | number | boolean | null
array ::= "[" space ( value ("," space value)* )? space "]"
number ::= ("-"? integral-part) ("." decimal-part)? ([eE] [-+]? integral-part)?
integral-part ::= [0] | [1-9] [0-9]{0,15}
decimal-part ::= [0-9]{1,16}
boolean ::= ("true" | "false")
null ::= "null"
`,
		},
		{
			"additional",
			`{"type":"object","properties":{"a":{"type":"number"}},"additionalProperties":{"type":"string"}}`,
			`root ::= object-5
space ::= | " " | "\n"{1,2} [ \t]{0,20}
number ::= ("-"? integral-part) ("." decimal-part)? ([eE] [-+]? integral-part)?
integral-part ::= [0] | [1-9] [0-9]{0,15}
decimal-part ::= [0-9]{1,16}
prop-1 ::= "\"a\"" space ":" space number
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
addk-2 ::= ["] ([a] char+ | [^"a] char* )? ["]
add-3 ::= addk-2 space ":" space string
rest-4 ::= ( "," space add-3 )*
object-5 ::= "{" space  (prop-1 rest-4 | add-3 ( "," space add-3 )* )? space "}"
`,
		},
		{
			"reqoptadd",
			`{"type":"object","properties":{"and":{"type":"number"},"also":{"type":"number"}},"required":["and"],"additionalProperties":{"type":"number"}}`,
			`root ::= object-6
space ::= | " " | "\n"{1,2} [ \t]{0,20}
number ::= ("-"? integral-part) ("." decimal-part)? ([eE] [-+]? integral-part)?
integral-part ::= [0] | [1-9] [0-9]{0,15}
decimal-part ::= [0-9]{1,16}
prop-1 ::= "\"also\"" space ":" space number
prop-2 ::= "\"and\"" space ":" space number
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
addk-3 ::= ["] ([a] ([l] ([s] ([o] char+ | [^"o] char*) | [^"s] char*) | [n] ([d] char+ | [^"d] char*) | [^"ln] char*) | [^"a] char* )? ["]
add-4 ::= addk-3 space ":" space number
rest-5 ::= ( "," space add-4 )*
object-6 ::= "{" space prop-2 ( "," space ( prop-1 rest-5 | add-4 ( "," space add-4 )* ) )? space "}"
`,
		},
		{
			"optional",
			`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"},"c":{"type":"string"}},"additionalProperties":false}`,
			`root ::= object-7
space ::= | " " | "\n"{1,2} [ \t]{0,20}
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
prop-1 ::= "\"a\"" space ":" space string
prop-2 ::= "\"b\"" space ":" space string
prop-3 ::= "\"c\"" space ":" space string
rest-4 ::= ( "," space prop-3 )?
rest-5 ::= ( "," space prop-2 )? rest-4
rest-6 ::= ( "," space prop-3 )?
object-7 ::= "{" space  (prop-1 rest-5 | prop-2 rest-6 | prop-3 )? space "}"
`,
		},
		{
			"uuid",
			`{"type":"string","format":"uuid"}`,
			`root ::= uuid
space ::= | " " | "\n"{1,2} [ \t]{0,20}
uuid ::= "\"" [0-9a-fA-F]{8} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{12} "\""
`,
		},
		{
			"date",
			`{"format":"date"}`,
			`root ::= fmt-1
space ::= | " " | "\n"{1,2} [ \t]{0,20}
date ::= [0-9]{4} "-" ( "0" [1-9] | "1" [0-2] ) "-" ( "0" [1-9] | [1-2] [0-9] | "3" [0-1] )
fmt-1 ::= "\"" date "\""
`,
		},
		{
			"tupledate",
			`{"items":[{"format":"date"},{"format":"uuid"},{"format":"time"},{"format":"date-time"}]}`,
			`root ::= array-4
space ::= | " " | "\n"{1,2} [ \t]{0,20}
date ::= [0-9]{4} "-" ( "0" [1-9] | "1" [0-2] ) "-" ( "0" [1-9] | [1-2] [0-9] | "3" [0-1] )
fmt-1 ::= "\"" date "\""
uuid ::= "\"" [0-9a-fA-F]{8} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{4} "-" [0-9a-fA-F]{12} "\""
time ::= ([01] [0-9] | "2" [0-3]) ":" [0-5] [0-9] ":" [0-5] [0-9] ( "." [0-9]{3} )? ( "Z" | ( "+" | "-" ) ( [01] [0-9] | "2" [0-3] ) ":" [0-5] [0-9] )
fmt-2 ::= "\"" time "\""
date-time ::= date "T" time
fmt-3 ::= "\"" date-time "\""
array-4 ::= "[" space fmt-1 "," space uuid "," space fmt-2 "," space fmt-3 space "]"
`,
		},
		{
			"allof",
			`{"allOf":[{"properties":{"a":{"type":"number"}}},{"properties":{"b":{"type":"number"}}}]}`,
			`root ::= object-3
space ::= | " " | "\n"{1,2} [ \t]{0,20}
number ::= ("-"? integral-part) ("." decimal-part)? ([eE] [-+]? integral-part)?
integral-part ::= [0] | [1-9] [0-9]{0,15}
decimal-part ::= [0-9]{1,16}
prop-1 ::= "\"a\"" space ":" space number
prop-2 ::= "\"b\"" space ":" space number
object-3 ::= "{" space prop-1 "," space prop-2 space "}"
`,
		},
		{
			"ref",
			`{"$ref":"#/definitions/foo","definitions":{"foo":{"type":"object","properties":{"a":{"type":"string"}},"required":["a"],"additionalProperties":false}}}`,
			`root ::= object-2
space ::= | " " | "\n"{1,2} [ \t]{0,20}
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
prop-1 ::= "\"a\"" space ":" space string
object-2 ::= "{" space prop-1 space "}"
`,
		},
		{
			"nullable",
			`{"type":["array","null"],"items":{"type":"string"}}`,
			`root ::= union-2
space ::= | " " | "\n"{1,2} [ \t]{0,20}
string ::= "\"" char* "\""
char ::= [^"\\\x7F\x00-\x1F] | [\\] (["\\bfnrt] | "u" [0-9a-fA-F]{4})
array-1 ::= "[" space (string ("," space string)*)? space "]"
null ::= "null"
union-2 ::= array-1 | null
`,
		},
		{
			"constesc",
			`{"type":"object","properties":{"code":{"const":" \r \n \" \\ "}},"required":["code"]}`,
			`root ::= object-3
space ::= | " " | "\n"{1,2} [ \t]{0,20}
lit-1 ::= "\" \\r \\n \\\" \\\\ \""
prop-2 ::= "\"code\"" space ":" space lit-1
object-3 ::= "{" space prop-2 space "}"
`,
		},
		{
			"max0",
			`{"type":"array","items":{"type":"boolean"},"maxItems":0}`,
			`root ::= array-1
space ::= | " " | "\n"{1,2} [ \t]{0,20}
boolean ::= ("true" | "false")
array-1 ::= "[" space  space "]"
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gbnfFor(t, tc.schema)
			if got != tc.want {
				t.Errorf("grammar mismatch\n got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestJSONSchemaToGBNFIntBounds(t *testing.T) {
	// Root rule content for integer minimum/maximum schemas, matching
	// llama.cpp's tests/test-json-schema-to-grammar.cpp expected values
	// (the outermost group wraps the alternatives, which is semantically
	// equivalent and required to bind the " | " alternatives together).
	cases := []struct {
		name   string
		schema string
		want   string
	}{
		{"min0", `{"type":"integer","minimum":0}`, `([0] | [1-9] [0-9]{0,15})`},
		{"min1", `{"type":"integer","minimum":1}`, `([1-9] [0-9]{0,15})`},
		{"min3", `{"type":"integer","minimum":3}`, `([1-2] [0-9]{1,15} | [3-9] [0-9]{0,15})`},
		{"min9", `{"type":"integer","minimum":9}`, `([1-8] [0-9]{1,15} | [9] [0-9]{0,15})`},
		{"min10", `{"type":"integer","minimum":10}`, `([1] ([0-9]{1,15}) | [2-9] [0-9]{1,15})`},
		{"min25", `{"type":"integer","minimum":25}`, `([1] [0-9]{2,15} | [2] ([0-4] [0-9]{1,14} | [5-9] [0-9]{0,14}) | [3-9] [0-9]{1,15})`},
		{"minneg5", `{"type":"integer","minimum":-5}`, `("-" ([0-5]) | [0] | [1-9] [0-9]{0,15})`},
		{"max1", `{"type":"integer","maximum":1}`, `("-" [1-9] [0-9]{0,15} | [0-1])`},
		{"max30", `{"type":"integer","maximum":30}`, `("-" [1-9] [0-9]{0,15} | [0-9] | ([1-2] [0-9] | [3] "0"))`},
		{"max100", `{"type":"integer","maximum":100}`, `("-" [1-9] [0-9]{0,15} | [0-9] | ([1-8] [0-9] | [9] [0-9]) | "100")`},
		{"maxneg5", `{"type":"integer","maximum":-5}`, `("-" ([0-4] [0-9]{1,15} | [5-9] [0-9]{0,15}))`},
		{"range15-300", `{"type":"integer","minimum":15,"maximum":300}`, `(([1] ([5-9]) | [2-9] [0-9]) | ([1-2] [0-9]{2} | [3] "00"))`},
		{"range-123-42", `{"type":"integer","minimum":-123,"maximum":42}`, `("-" ([0-9] | ([1-8] [0-9] | [9] [0-9]) | "1" ([0-1] [0-9] | [2] [0-3])) | [0-9] | ([1-3] [0-9] | [4] [0-2]))`},
		{"range5-30", `{"type":"integer","minimum":5,"maximum":30}`, `([5-9] | ([1-2] [0-9] | [3] "0"))`},
		{"range0-23", `{"type":"integer","minimum":0,"maximum":23}`, `([0-9] | ([1] [0-9] | [2] [0-3]))`},
		{"range-10-10", `{"type":"integer","minimum":-10,"maximum":10}`, `("-" ([0-9] | "10") | [0-9] | "10")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gbnfFor(t, tc.schema)
			rules := gbnfRules(got)
			root := rules["root"]
			content, ok := rules[root]
			if !ok {
				t.Fatalf("root references undefined rule %q\nfull grammar:\n%s", root, got)
			}
			if content != tc.want {
				t.Errorf("rule %q mismatch\n got: %s\nwant: %s", root, content, tc.want)
			}
		})
	}
}

func TestJSONSchemaToGBNFErrors(t *testing.T) {
	cases := []struct {
		name   string
		schema string
	}{
		{"remote-ref", `{"$ref":"https://example.com/schema#"}`},
		{"unresolved-ref", `{"$ref":"#/definitions/nope","definitions":{}}`},
		{"unknown-type", `{"type":"bogus"}`},
		{"oneof-not-array", `{"oneOf":{"type":"string"}}`},
		{"anyof-not-array", `{"anyOf":{"type":"string"}}`},
		{"nonobject-node", `"string"`},
		{"bad-type-entry", `{"type":["string",42]}`},
		{"invalid-bounds", `{"type":"integer","minimum":10,"maximum":5}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var schema any
			if err := json.Unmarshal([]byte(tc.schema), &schema); err != nil {
				t.Fatalf("invalid schema JSON: %v", err)
			}
			if _, err := jsonSchemaToGBNF(schema); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestGBNFRuleConsistency(t *testing.T) {
	// Every rule name referenced by another rule must be defined.
	cases := map[string]string{
		"simple":    `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}`,
		"optional":  `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"},"c":{"type":"string"}},"additionalProperties":false}`,
		"reqoptadd": `{"type":"object","properties":{"and":{"type":"number"},"also":{"type":"number"}},"required":["and"],"additionalProperties":{"type":"number"}}`,
		"tupledate": `{"items":[{"format":"date"},{"format":"uuid"},{"format":"time"},{"format":"date-time"}]}`,
		"enum":      `{"type":"string","enum":["red","green","blue"]}`,
		"constesc":  `{"type":"object","properties":{"code":{"const":" \r \n \" \\ "}},"required":["code"]}`,
		"empty":     `{}`,
		"ref":       `{"$ref":"#/definitions/foo","definitions":{"foo":{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}}}`,
	}
	for name, sch := range cases {
		t.Run(name, func(t *testing.T) {
			rules := gbnfRules(gbnfFor(t, sch))
			referenced := map[string]bool{}
			for _, content := range rules {
				for _, tok := range strings.Fields(content) {
					if _, ok := rules[tok]; ok {
						referenced[tok] = true
					}
				}
			}
			for r := range referenced {
				if _, ok := rules[r]; !ok {
					t.Errorf("rule %q referenced but not defined", r)
				}
			}
		})
	}
}

func TestUnknownFormatDegradesToPlainString(t *testing.T) {
	// Like llama.cpp, unknown string formats are ignored.
	got := gbnfFor(t, `{"type":"string","format":"bogus"}`)
	if !strings.Contains(got, `string ::= "\"" char* "\""`) {
		t.Errorf("expected plain string rule, got:\n%s", got)
	}
}

func TestGrammarFromResponseFormat(t *testing.T) {
	ok := `{"type":"json_schema","json_schema":{"name":"X","schema":{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}}}`
	got, err := grammarFromResponseFormat(mustJSON(t, ok))
	if err != nil {
		t.Fatalf("grammarFromResponseFormat: %v", err)
	}
	rules := gbnfRules(got)
	if rules["root"] == "" {
		t.Errorf("missing root rule:\n%s", got)
	}
	if !strings.Contains(got, `"\"a\"" space ":" space string`) {
		t.Errorf("missing property a rule:\n%s", got)
	}

	bad := []any{
		map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "X"}},
		map[string]any{"type": "text"},
		"not an object",
		nil,
	}
	for _, rf := range bad {
		if _, err := grammarFromResponseFormat(rf); err == nil {
			t.Errorf("grammarFromResponseFormat(%v): expected error, got nil", rf)
		}
	}
}

func TestSchemaObjectFromResponseFormat(t *testing.T) {
	rf := map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"schema": map[string]any{"type": "object"},
		},
	}
	schema := schemaObjectFromResponseFormat(rf)
	if _, ok := schema.(map[string]any); !ok {
		t.Errorf("expected schema object, got %#v", schema)
	}
	if schemaObjectFromResponseFormat(map[string]any{"type": "text"}) != nil {
		t.Errorf("expected nil for non-json_schema response format")
	}
	if schemaObjectFromResponseFormat(nil) != nil {
		t.Errorf("expected nil for nil response format")
	}
}

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return v
}
