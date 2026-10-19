package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseExpect parses one expect line inside a minimal request and returns the
// resulting assertion (or the parse error).
func parseExpect(t *testing.T, line string) (*Assertion, error) {
	t.Helper()
	f, err := Parse("GET http://x/\n\n>>>\n"+line+"\n<<<\n", "t.http")
	if err != nil {
		return nil, err
	}
	require.Len(t, f.Requests, 1)
	require.Len(t, f.Requests[0].Assertions, 1, "line %q", line)
	return f.Requests[0].Assertions[0], nil
}

func TestExpectValues(t *testing.T) {
	tests := []struct {
		line     string
		subject  string
		op       AssertionOperator
		expected any
	}{
		{`expect status 200`, "status", OpEquals, 200},
		{`expect status == 200`, "status", OpEquals, 200},
		{`expect status==200`, "status", OpEquals, 200},
		{`expect status 200   # shorthand`, "status", OpEquals, 200},
		{`expect duration < 1.5`, "duration", OpLessThan, 1.5},
		{`expect body.n == -3`, "body.n", OpEquals, -3},
		{`expect body.ok == true`, "body.ok", OpEquals, true},
		{`expect body.ok == True`, "body.ok", OpEquals, true},
		{`expect body.x == null`, "body.x", OpEquals, nil},
		{`expect body.x exists`, "body.x", OpExists, nil},
		{`expect body.x !exists  # gone`, "body.x", OpNotExists, nil},
		// Unquoted values are the rest of the line, verbatim.
		{`expect body.name == Jane Doe`, "body.name", OpEquals, "Jane Doe"},
		{`expect body.p == ./x`, "body.p", OpEquals, "./x"},
		{`expect body.p == ^x`, "body.p", OpEquals, "^x"},
		{`expect body.p == $x`, "body.p", OpEquals, "$x"},
		{`expect body.p == @x`, "body.p", OpEquals, "@x"},
		{`expect body.p == #fff`, "body.p", OpEquals, "#fff"},
		{`expect body.t == Issue #5`, "body.t", OpEquals, "Issue #5"},
		{`expect body.t == foo # note`, "body.t", OpEquals, "foo"},
		{`expect body.id == 123abc`, "body.id", OpEquals, "123abc"},
		{`expect body.id == {{userId}}`, "body.id", OpEquals, "{{userId}}"},
		{`expect body.name Jane`, "body.name", OpEquals, nil}, // replaced below: error case
	}
	for _, tt := range tests {
		if tt.line == `expect body.name Jane` {
			continue
		}
		t.Run(tt.line, func(t *testing.T) {
			a, err := parseExpect(t, tt.line)
			require.NoError(t, err)
			assert.Equal(t, tt.subject, a.Subject)
			assert.Equal(t, tt.op, a.Operator)
			assert.Equal(t, tt.expected, a.Expected)
		})
	}
}

func TestExpectStringsAndLists(t *testing.T) {
	a, err := parseExpect(t, `expect body.m == "line\nbreak \"q\""`)
	require.NoError(t, err)
	assert.Equal(t, "line\nbreak \"q\"", a.Expected)

	a, err = parseExpect(t, `expect body.m == 'it\'s # fine'  # c`)
	require.NoError(t, err)
	assert.Equal(t, "it's # fine", a.Expected)

	// Not valid JSON escapes: kept as written (regex-friendly).
	a, err = parseExpect(t, `expect body.m matches "^\d+$"`)
	require.NoError(t, err)
	assert.Equal(t, `^\d+$`, a.Expected)

	a, err = parseExpect(t, `expect status in [200, 201,204]`)
	require.NoError(t, err)
	assert.Equal(t, OpIn, a.Operator)
	assert.Equal(t, []any{200, 201, 204}, a.Expected)

	a, err = parseExpect(t, `expect body.role !in ["admin", editor, {{r}}, 'a,b']  # c`)
	require.NoError(t, err)
	assert.Equal(t, OpNotIn, a.Operator)
	assert.Equal(t, []any{"admin", "editor", "{{r}}", "a,b"}, a.Expected)

	a, err = parseExpect(t, `expect body.o == {"a": 1, "b": [1.5, "x"], "c": {"d": null}}`)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": 1, "b": []any{1.5, "x"}, "c": map[string]any{"d": nil}}, a.Expected)

	a, err = parseExpect(t, `expect body.e == []`)
	require.NoError(t, err)
	assert.Equal(t, []any{}, a.Expected)
}

func TestExpectSubjects(t *testing.T) {
	tests := map[string]string{
		`expect body.items.# == 3`:                             "body.items.#",
		`expect body.items.#(name=="Jane Doe").id == 1`:        "body.items.#(name==\"Jane Doe\").id",
		`expect body.users[0].name == "Admin"`:                 "body.users[0].name",
		`expect body[0].id exists`:                             "body[0].id",
		`expect header Content-Type contains json`:             "header Content-Type",
		`expect Header X-Id exists`:                            "header X-Id",
		`expect header exists`:                                 "header",
		`expect jsonpath $.users[0].id exists`:                 "jsonpath $.users[0].id",
		`expect jsonpath $.data.count > 0`:                     "jsonpath $.data.count",
		`expect user.name == "John"`:                           "user.name",
		`expect body.{{field}} == 1`:                           "body.{{field}}",
		`expect body.a.b.c.d.e.f exists # deep`:                "body.a.b.c.d.e.f",
		`expect body.items.#(tags.#(=="a")).id exists`:         "body.items.#(tags.#(==\"a\")).id",
		`expect body.items.#(price>10)#.name length > 1`:       "body.items.#(price>10)#.name",
		`expect body.title startsWith "Chapter #1"`:            "body.title",
		`expect duration<500`:                                  "duration",
		`expect p95 < 200`:                                     "p95",
		"expect\tstatus\t200":                                  "status",
		"   expect   body.id    exists   ":                     "body.id",
		`expect body.items[?(@.id == 1)].name exists`:          "body.items[?(@.id == 1)].name",
		`expect body.items.#(name=="a b").tags.# length >= 2 `: "body.items.#(name==\"a b\").tags.#",
	}
	for line, want := range tests {
		t.Run(line, func(t *testing.T) {
			a, err := parseExpect(t, line)
			require.NoError(t, err)
			assert.Equal(t, want, a.Subject)
		})
	}
}

func TestExpectOperators(t *testing.T) {
	ops := map[string]AssertionOperator{
		`expect body.a contains "x"`:        OpContains,
		`expect body.a !contains "x"`:       OpNotContains,
		`expect body.a startsWith "x"`:      OpStartsWith,
		`expect body.a STARTSWITH "x"`:      OpStartsWith,
		`expect body.a endsWith "x"`:        OpEndsWith,
		`expect body.a matches /x/`:         OpMatches,
		`expect body.a length 3`:            OpLength,
		`expect body.a length == 3`:         OpLength,
		`expect body.a length > 3`:          OpLengthGt,
		`expect body.a length >= 3`:         OpLengthGte,
		`expect body.a length < 3`:          OpLengthLt,
		`expect body.a length <= 3`:         OpLengthLte,
		`expect body.a includes "x"`:        OpIncludes,
		`expect body.a !includes "x"`:       OpNotIncludes,
		`expect body.a in [1]`:              OpIn,
		`expect body.a !in [1]`:             OpNotIn,
		`expect body.a type string`:         OpType,
		`expect body schema ./s.json`:       OpSchema,
		`expect body snapshot "name"`:       OpSnapshot,
		`expect body snapshot`:              OpSnapshot,
		`expect body.a != 1`:                OpNotEquals,
		`expect body.a >= 1`:                OpGreaterOrEqual,
		`expect body.a contains"x"`:         OpContains,
		`expect body.a exists`:              OpExists,
		`expect body.a each type object`:    OpEach,
		`expect body.a each > 0`:            OpEach,
		`expect body.a each body.id exists`: OpEach,
	}
	for line, want := range ops {
		t.Run(line, func(t *testing.T) {
			a, err := parseExpect(t, line)
			require.NoError(t, err)
			assert.Equal(t, want, a.Operator)
		})
	}
}

func TestExpectErrors(t *testing.T) {
	tests := []struct {
		line string
		want string
		col  int // 0 = don't check
	}{
		{`expect body.name contians "x"`, `unknown operator "contians" (did you mean "contains"?)`, 18},
		{`expect body.name startWith "x"`, `did you mean "startsWith"`, 0},
		{`expect body.name = "x"`, `unknown operator "=" (did you mean "=="?)`, 18},
		{`expect body.name === "x"`, `did you mean "=="`, 0},
		{`expect body.name equals "x"`, `did you mean "=="`, 0},
		{`expect body.name !== "x"`, `did you mean "!="`, 0},
		{`expect body.name Jane`, `unknown operator "Jane"`, 0},
		{`expect body.name`, `missing operator and value`, 0},
		{`expect body.name ==`, `operator == needs a value`, 0},
		{`expect body.name contains`, `operator contains needs a value`, 0},
		{`expect body.name == # only a comment`, `operator == needs a value`, 0},
		{`expect status 200 201`, `expect status in [200, 201]`, 0},
		{`expect status == ok`, `status is a number but "ok" is not`, 0},
		{`expect body.n > abc`, `operator > needs a number`, 0},
		{`expect body.n length abc`, `operator length needs a number`, 0},
		{`expect body.x exists yes`, `unexpected text "yes" after exists`, 0},
		{`expect body.x in 200`, `operator in needs a list`, 0},
		{`expect body.x type strng`, `did you mean "string"`, 0},
		{`expect body.x type foo`, `unknown type "foo"`, 0},
		{`expect body.x == "abc`, `unterminated string`, 0},
		{`expect body.x == "abc" def`, `unexpected text "def"`, 0},
		{`expect body.x == [1, 2`, `unterminated list`, 0},
		{`expect body.x == {"a": }`, `invalid JSON object`, 0},
		{`expect body.x matches /(/`, `invalid regular expression`, 0},
		{`expect body.x matches /a/z`, `unknown operator`, -1}, // placeholder, replaced below
		{`expect body.x matches /a/y`, `unsupported regex flag`, 0},
		{`expect body.x length != 3`, `length supports`, 0},
		{`expect body.x each`, `each needs an assertion`, 0},
		{`expect body.x each each exists`, `each cannot be nested`, 0},
		{`expect`, `missing subject`, 0},
		{`expcet status 200`, `did you mean "expect"`, 0},
		{`status 200`, `unexpected line in block`, 0},
		{`expect status ~ 200`, `unknown operator "~"`, 0},
	}
	for _, tt := range tests {
		if tt.col == -1 {
			continue
		}
		t.Run(tt.line, func(t *testing.T) {
			_, err := Parse("GET http://x/\n\n>>>\n"+tt.line+"\n<<<\n", "t.http")
			require.Error(t, err)
			pe, ok := err.(*ParseError)
			require.True(t, ok, "%T", err)
			assert.Contains(t, pe.Message, tt.want)
			assert.Equal(t, 4, pe.Line)
			assert.Equal(t, "t.http", pe.File)
			if tt.col > 0 {
				assert.Equal(t, tt.col, pe.Column)
			}
			assert.Equal(t, tt.line, pe.Snippet)
		})
	}
}

func TestExpectRegexLiteral(t *testing.T) {
	a, err := parseExpect(t, `expect body.id matches /^\d+$/`)
	require.NoError(t, err)
	assert.Equal(t, `^\d+$`, a.Expected)

	a, err = parseExpect(t, `expect body.id matches /^abc\/def$/i  # flags`)
	require.NoError(t, err)
	assert.Equal(t, `(?i)^abc\/def$`, a.Expected)

	a, err = parseExpect(t, `expect body.id matches /x/gim`)
	require.NoError(t, err)
	assert.Equal(t, `(?im)x`, a.Expected)

	// Only "matches" reads /.../ specially; a bare path stays a path.
	a, err = parseExpect(t, `expect body.url == /api/v1/users`)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/users", a.Expected)
	a, err = parseExpect(t, `expect body.url matches /api/v1/users`)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/users", a.Expected)

	a, err = parseExpect(t, `expect body.id matches /{{pattern}}/`)
	require.NoError(t, err)
	assert.Equal(t, "{{pattern}}", a.Expected)
}

func TestExpectEachForms(t *testing.T) {
	cases := map[string]EachSpec{
		`expect body.items each type object`:                             {Operator: OpType, Expected: "object"},
		`expect body.items each > 0`:                                     {Operator: OpGreaterThan, Expected: 0},
		`expect body.items each 5`:                                       {Operator: OpEquals, Expected: 5},
		`expect body.items each "x"`:                                     {Operator: OpEquals, Expected: "x"},
		`expect body.items each exists`:                                  {Operator: OpExists},
		`expect body.items each body.id exists`:                          {Path: "id", Operator: OpExists},
		`expect body.items each id exists`:                               {Path: "id", Operator: OpExists},
		`expect body.items each.id exists`:                               {Path: "id", Operator: OpExists},
		`expect body.items each body.email matches /^.+@.+$/`:            {Path: "email", Operator: OpMatches, Expected: "^.+@.+$"},
		`expect body.items each body.n >= 1`:                             {Path: "n", Operator: OpGreaterOrEqual, Expected: 1},
		`expect body.items each tags[0] == "a"`:                          {Path: "tags[0]", Operator: OpEquals, Expected: "a"},
		`expect body.items each {"operator": "type", "value": "object"}`: {Operator: OpType, Expected: "object"},
		`expect body.items each {"operator": "equals", "value": 3}`:      {Operator: OpEquals, Expected: 3},
	}
	for line, want := range cases {
		t.Run(line, func(t *testing.T) {
			a, err := parseExpect(t, line)
			require.NoError(t, err)
			require.Equal(t, OpEach, a.Operator)
			spec, ok := a.Expected.(*EachSpec)
			require.True(t, ok, "%T", a.Expected)
			assert.Equal(t, want, *spec)
		})
	}
}

func TestAssertionBlockStrictness(t *testing.T) {
	// A comment and a blank line inside the block are fine.
	f, err := Parse("GET http://x/\n\n>>>\n# note\n// note\n\nexpect status 200\n<<<\n", "t.http")
	require.NoError(t, err)
	require.Len(t, f.Requests[0].Assertions, 1)
	assert.Equal(t, 7, f.Requests[0].Assertions[0].Line)

	// CRLF line endings.
	f, err = Parse("GET http://x/\r\n\r\n>>>\r\nexpect status 200\r\nexpect body.a == Jane Doe\r\n<<<\r\n", "t.http")
	require.NoError(t, err)
	require.Len(t, f.Requests[0].Assertions, 2)
	assert.Equal(t, "Jane Doe", f.Requests[0].Assertions[1].Expected)

	// Two assertion blocks both count.
	f, err = Parse("GET http://x/\n\n>>>\nexpect status 200\n<<<\n\n>>>\nexpect body.a exists\n<<<\n", "t.http")
	require.NoError(t, err)
	assert.Len(t, f.Requests[0].Assertions, 2)

	// A new request after an unclosed block is an unclosed-block error.
	_, err = Parse("GET http://x/\n\n>>>\nexpect status 200\n### two\nGET http://y/\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed")

	// Orphan and misspelt blocks are errors, not silently skipped.
	_, err = Parse("GET http://x/\n\n>>>captur\ntoken from body.t\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `did you mean >>>capture`)
	_, err = Parse(">>>\nexpect status 200\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not inside a request")
	_, err = Parse("GET http://x/\n\n>>>db {{url}}\nSELECT 1\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "takes no arguments")
}

func TestCaptureBlock(t *testing.T) {
	src := "GET http://x/\n\n>>>capture\n" +
		"id from body.id\n" +
		"first from body.items[0].name   # comment\n" +
		"n from body.items.#\n" +
		"adm from body.users.#(role==\"admin\").name\n" +
		"all from body\n" +
		"idx from body[2].x\n" +
		"jp from jsonpath $.a[*].b\n" +
		"rid from header X-Request-Id\n" +
		"code from status\n" +
		"ms from duration\n" +
		"bare from data.token\n" +
		"<<<\n"
	f, err := Parse(src, "t.http")
	require.NoError(t, err)
	var got []string
	for _, c := range f.Requests[0].Captures {
		got = append(got, c.Name+"|"+c.Source.String()+"|"+c.Path)
	}
	assert.Equal(t, []string{
		"id|body|id",
		"first|body|items[0].name",
		"n|body|items.#",
		`adm|body|users.#(role=="admin").name`,
		"all|body|",
		"idx|body|[2].x",
		"jp|body|a.#.b",
		"rid|header|X-Request-Id",
		"code|status|",
		"ms|duration|",
		"bare|body|data.token",
	}, got)
	assert.Empty(t, f.Warnings)

	_, err = Parse("GET http://x/\n\n>>>capture\ntoken body.t\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expected "<name> from <source>"`)

	f, err = Parse("GET http://x/\n\n>>>capture\ntoken from body.token   <-- note\n<<<\n", "t.http")
	require.NoError(t, err)
	require.Len(t, f.Warnings, 1)
	assert.Contains(t, f.Warnings[0].Message, "contains spaces")
}

func TestDBBlock(t *testing.T) {
	src := "# @db sqlite://x.db\nGET http://x/\n\n>>>db\n" +
		"SELECT name, \"quoted col\" FROM users WHERE id = {{userId}} AND n = 'a b';\n" +
		"expect name == \"Jane Doe\"\n" +
		"expect active 1\n" +
		"query SELECT count(*) as c\n" +
		"  FROM users\n" +
		"expect c > 0 # at least one\n" +
		"<<<\n"
	f, err := Parse(src, "t.http")
	require.NoError(t, err)
	db := f.Requests[0].DBAssertions
	require.Len(t, db, 3)
	assert.Equal(t, `SELECT name, "quoted col" FROM users WHERE id = {{userId}} AND n = 'a b';`, db[0].Query)
	assert.Equal(t, "Jane Doe", db[0].Expected)
	assert.Equal(t, 1, db[1].Expected)
	assert.Equal(t, OpEquals, db[1].Operator)
	assert.Equal(t, "SELECT count(*) as c\nFROM users", db[2].Query)
	assert.Equal(t, OpGreaterThan, db[2].Operator)
	assert.Equal(t, 0, db[2].Expected)

	_, err = Parse("GET http://x/\n\n>>>db\nexpect c > 0\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expect before any query")

	_, err = Parse("GET http://x/\n\n>>>db\nSELECT 1 AS c\nexpect c contians 1\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did you mean")

	_, err = Parse("GET http://x/\n\n>>>db\nSELECT 1 AS c\nexpect c matches /x/\n<<<\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported in >>>db")
}

func TestDirectiveWarnings(t *testing.T) {
	src := "# @tag smoke\n# @nmae x\n# @x-owner me\n# @contract.state s\n# @stress.weight 2\n# @timeout abc\n# @timeout {{t}}\nGET http://x/\n"
	f, err := Parse(src, "t.http")
	require.NoError(t, err)
	var msgs []string
	for _, w := range f.Warnings {
		msgs = append(msgs, w.String())
	}
	require.Len(t, msgs, 3, "%v", msgs)
	assert.Contains(t, msgs[0], `t.http:1:1: unknown directive @tag (did you mean @tags?)`)
	assert.Contains(t, msgs[1], `unknown directive @nmae (did you mean @name?)`)
	assert.Contains(t, msgs[2], `invalid @timeout value "abc"`)
	assert.False(t, strings.Contains(strings.Join(msgs, "\n"), "{{t}}"))
}

// FuzzBlockLines feeds arbitrary text to the raw-line block readers (assertion,
// capture, db). They must never panic, and a failure must be a *ParseError
// whose position points inside the input.
func FuzzBlockLines(f *testing.F) {
	for _, seed := range []string{
		"expect status 200",
		"expect body.items.#(name==\"Jane Doe\").id == 1 # c",
		"expect body.x matches /a\\/b/i",
		"expect body.x in [1, \"a,b\", {{v}}, [2]]",
		"expect body.x each body.id exists",
		"expect body.x each {\"operator\":\"type\",\"value\":\"object\"}",
		"token from body.items[0].name",
		"SELECT 1 AS c\nexpect c == 1",
		"expect header X-Id exists\r\nexpect status 200\r\n",
		"expect jsonpath $.a[*].b length > 1",
		"\x00\xff expect",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		for _, kind := range []string{"", "capture", "db"} {
			src := "GET http://x/\n\n>>>" + kind + "\n" + body + "\n<<<\n"
			_, err := Parse(src, "f.http")
			if err == nil {
				continue
			}
			pe, ok := err.(*ParseError)
			if !ok {
				t.Fatalf("error is %T, want *ParseError: %v", err, err)
			}
			if pe.Line < 1 || pe.Column < 0 {
				t.Fatalf("bad position %d:%d for %q", pe.Line, pe.Column, src)
			}
			_ = pe.Error() // must not panic building the caret line
		}
	})
}

func TestShellBlockIsRaw(t *testing.T) {
	src := "GET http://x/\n\n>>>shell\n# a comment\necho \"a  b\" 'c d' # keep\ncurl -H \"X: {{tok}}\" http://h\n\n<<<\n"
	f, err := Parse(src, "t.http")
	require.NoError(t, err)
	var got []string
	for _, c := range f.Requests[0].ShellCommands {
		got = append(got, c.Command)
	}
	assert.Equal(t, []string{`echo "a  b" 'c d' # keep`, `curl -H "X: {{tok}}" http://h`}, got)
	assert.Equal(t, 5, f.Requests[0].ShellCommands[0].Line)

	_, err = Parse("GET http://x/\n\n>>>shell\necho hi\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed >>>shell")
}
