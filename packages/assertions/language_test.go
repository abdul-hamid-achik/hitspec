package assertions

import (
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// run parses one expect line (so the test covers parser + evaluator together)
// and evaluates it against resp.
func run(t *testing.T, resp *http.Response, line string) *Result {
	t.Helper()
	f, err := parser.Parse("GET http://x/\n\n>>>\n"+line+"\n<<<\n", "t.http")
	require.NoError(t, err, line)
	require.Len(t, f.Requests[0].Assertions, 1, line)
	return NewEvaluator(resp).Evaluate(f.Requests[0].Assertions[0])
}

func resp(contentType, body string) *http.Response {
	r := createResponse(200, body, map[string]string{"Content-Type": contentType})
	return r
}

func expectPass(t *testing.T, r *http.Response, lines ...string) {
	t.Helper()
	for _, l := range lines {
		res := run(t, r, l)
		assert.True(t, res.Passed, "%s -> %s", l, res.Message)
	}
}

func expectFail(t *testing.T, r *http.Response, lines ...string) {
	t.Helper()
	for _, l := range lines {
		res := run(t, r, l)
		assert.False(t, res.Passed, "%s unexpectedly passed", l)
	}
}

func TestNonJSONBodyPathIsNotPresent(t *testing.T) {
	html := resp("text/html", "<html>hello</html>")
	// Item 7: a path on a non-JSON body used to read the whole body, so
	// "exists" always passed.
	res := run(t, html, "expect body.x exists")
	assert.False(t, res.Passed)
	assert.Contains(t, res.Message, "not JSON")
	expectFail(t, html, "expect body.x !exists", "expect body[0] exists", "expect jsonpath $.x exists", "expect x exists")
	// The whole body is still readable.
	expectPass(t, html, `expect body contains "hello"`, "expect body startsWith <html>", `expect body type string`, "expect body exists")
}

func TestJSONMediaTypes(t *testing.T) {
	for _, ct := range []string{
		"application/json",
		"application/json; charset=utf-8",
		"Application/JSON;charset=UTF-8",
		"application/problem+json",
		"application/problem+json; charset=utf-8",
		"application/vnd.api+json",
		"application/ld+json",
		"text/json",
	} {
		expectPass(t, resp(ct, `{"title":"x","n":[1,2]}`), `expect body.title == x`, "expect body.n.# == 2", "expect body.n[1] == 2")
	}
	// text/plain is not JSON even when the text looks like it.
	expectFail(t, resp("text/plain", `{"a":1}`), "expect body.a exists")
	expectFail(t, resp("", `{"a":1}`), "expect body.a exists")
	// A JSON content type with a broken body is an error, not "absent".
	res := run(t, resp("application/json", `{oops`), "expect body.a !exists")
	assert.False(t, res.Passed)
	assert.Contains(t, res.Message, "not valid JSON")
}

func TestNullVersusAbsent(t *testing.T) {
	r := resp("application/json", `{"n":null,"s":"x","z":0,"e":"","o":{"a":null}}`)
	expectPass(t, r,
		"expect body.n type null",
		"expect body.n == null",
		"expect body.n !exists",
		"expect body.missing !exists",
		"expect body.s != null",
		"expect body.s exists",
		"expect body.z exists",
		"expect body.e exists",
		"expect body.o.a == null",
		"expect body.s type string",
	)
	expectFail(t, r,
		"expect body.n exists",
		"expect body.n != null",
		"expect body.s == null",
		"expect body.s type null",
		// absent is not null: a misspelt path must not satisfy == null / != null
		"expect body.missing == null",
		"expect body.missing != null",
		"expect body.missing type null",
		"expect body.missing exists",
		"expect body.missing == 1",
		"expect body.missing > 1",
	)
	// negations hold for an absent path
	expectPass(t, r, `expect body.missing != 1`, `expect body.missing !contains x`, `expect body.missing !in [1]`)
	// the failure explains itself
	res := run(t, r, "expect body.missing == null")
	assert.Contains(t, res.Message, "does not exist")
}

func TestObjectAndArrayEquality(t *testing.T) {
	r := resp("application/json", `{"obj":{"a":1,"b":[1,2.5,"x"],"c":{"d":null}},"arr":[1,2,3],"empty":[],"eo":{}}`)
	expectPass(t, r,
		`expect body.obj == {"a":1,"b":[1,2.5,"x"],"c":{"d":null}}`,
		`expect body.obj == {"c":{"d":null},"a":1.0,"b":[1.0,2.5,"x"]}`,
		`expect body.arr == [1, 2, 3]`,
		`expect body.arr == [1.0,2,3]`,
		`expect body.arr != [3, 2, 1]`,
		`expect body.obj != {"a":1}`,
		`expect body.empty == []`,
		`expect body.eo == {}`,
		`expect body == {"obj":{"a":1,"b":[1,2.5,"x"],"c":{"d":null}},"arr":[1,2,3],"empty":[],"eo":{}}`,
	)
	expectFail(t, r,
		`expect body.obj == {"a":2,"b":[1,2.5,"x"],"c":{"d":null}}`,
		`expect body.arr == [1, 2]`,
		`expect body.arr == [1, 2, "3"]`,
		`expect body.arr == "1,2,3"`,
		`expect body.obj == {"a":1}`,
	)
	// composites never match through string coercion
	expectFail(t, resp("application/json", `{"l":["a b"]}`), `expect body.l == ["a", "b"]`)
}

func TestLengthCountsRunes(t *testing.T) {
	r := resp("application/json", `{"s":"héllo wörld","cjk":"日本語","emoji":"😀😀","a":[1,2],"o":{"k":1}}`)
	expectPass(t, r,
		"expect body.s length 11", "expect body.cjk length 3", "expect body.emoji length 2",
		"expect body.cjk length <= 3", "expect body.a length 2", "expect body.o length 1",
	)
	expectFail(t, r, "expect body.cjk length 9", "expect body.n length 1")
	expectPass(t, resp("text/plain", "日本語"), "expect body length 3")
}

func TestStringOperatorsOnJSON(t *testing.T) {
	body := `{"name":"Jane","tags":["a","b"],"o":{"k":"v"},"n":12,"f":1.5,"big":123456789012}`
	r := resp("application/json", body)
	expectPass(t, r,
		// the whole JSON body is matched as its raw text, not Go's map[...] form
		`expect body contains "Jane"`,
		`expect body contains "\"name\":\"Jane\""`,
		`expect body startsWith {`,
		`expect body endsWith }`,
		`expect body !contains map[`,
		`expect body matches /^\{.*\}$/`,
		// objects and arrays are JSON text
		`expect body.o contains {"k":"v"}`,
		`expect body.o !contains map[`,
		`expect body.tags startsWith ["a"`,
		`expect body.tags contains "b"`,
		`expect body.tags endsWith "b"]`,
		// scalars
		`expect body.n contains 2`,
		`expect body.f startsWith 1.`,
		`expect body.big contains 123456789012`,
		`expect body.name matches /^J.*e$/`,
		`expect body.name matches /^j/i`,
	)
	expectFail(t, r, `expect body.o contains map`, `expect body contains map[`)
}

func TestJSONPathSubject(t *testing.T) {
	r := resp("application/json", `{"users":[{"id":7,"name":"a","tags":["x"]},{"id":8,"name":"b","tags":[]}],"data":{"count":2,"a.b":3}}`)
	expectPass(t, r,
		"expect jsonpath $.users[0].id == 7",
		"expect jsonpath $.users[1].name == b",
		"expect jsonpath $.users[0].id exists",
		"expect jsonpath $.data.count > 0",
		"expect jsonpath $.users[*].name == [\"a\", \"b\"]",
		"expect jsonpath $.users[*].id includes 8",
		"expect jsonpath $.users[*] length 2",
		"expect jsonpath $.users length 2",
		"expect jsonpath $.users[0].tags[0] == x",
		"expect jsonpath $['data']['a.b'] == 3",
		"expect jsonpath $.users[5] !exists",
		"expect jsonpath $.nope !exists",
		"expect jsonpath $ type object",
	)
	expectFail(t, r, "expect jsonpath $.users[0].id == 8", "expect jsonpath $.users[9].id exists")
	// unsupported JSONPath is an error, not a guess
	res := run(t, r, "expect jsonpath $..id exists")
	assert.False(t, res.Passed)
	assert.Contains(t, res.Message, "recursive descent")
	res = run(t, r, "expect jsonpath $.users[?(@.id==7)] exists")
	assert.False(t, res.Passed)
	assert.Contains(t, res.Message, "not supported")
}

func TestEachForms(t *testing.T) {
	r := resp("application/json", `{"items":[{"id":1,"email":"a@x.io","n":3,"tags":["t"]},{"id":2,"email":"b@x.io","n":4,"tags":["u"]}],"nums":[1,2,3],"none":[],"strs":["a","b"]}`)
	expectPass(t, r,
		"expect body.items each type object",
		"expect body.items each body.id exists",
		"expect body.items each id exists",
		"expect body.items each.id exists",
		`expect body.items each body.email matches /^.+@.+\..+$/`,
		"expect body.items each body.n > 2",
		"expect body.items each n >= 3",
		"expect body.items each n < 10",
		`expect body.items each tags[0] matches /^[tu]$/`,
		"expect body.items each tags length 1",
		"expect body.items each id in [1, 2, 3]",
		"expect body.items each missing !exists",
		"expect body.nums each > 0",
		"expect body.nums each type number",
		"expect body.nums each in [1, 2, 3]",
		"expect body.nums each != 9",
		`expect body.strs each type string`,
		`expect body.strs each matches /^[ab]$/`,
		`expect body.items each {"operator": "type", "value": "object"}`,
		`expect body.nums each {"operator": ">", "value": 0}`,
		"expect body.none each type object", // empty: vacuous
	)
	expectFail(t, r,
		"expect body.items each type string",
		"expect body.items each id == 1",
		"expect body.items each missing exists",
		"expect body.nums each > 1",
		"expect body.nums each == 2",
		"expect body.strs each type number",
		"expect body.nope each exists",
		"expect body.items each n > 3",
	)
	res := run(t, r, "expect body.nums each > 1")
	assert.Contains(t, res.Message, "item[0]")
	res = run(t, r, "expect body.items each id == 1")
	assert.Contains(t, res.Message, "item[1]")
	res = run(t, resp("application/json", `{"a":5}`), "expect body.a each exists")
	assert.False(t, res.Passed)
	assert.Contains(t, res.Message, "array")
}

func TestCountAndQueries(t *testing.T) {
	r := resp("application/json", `{"items":[{"name":"Jane Doe","id":1,"tags":["a","b"]},{"name":"Bob","id":2,"tags":[]}],"matrix":[[1,2],[3]]}`)
	expectPass(t, r,
		"expect body.items.# == 2",
		"expect body.items.# >= 1",
		"expect body.items.#(id==2).name == Bob",
		`expect body.items.#(name=="Jane Doe").id == 1`,
		`expect body.items.#(name=="Jane Doe").tags.# == 2`,
		`expect body.items.#(id>=1)#.name == ["Jane Doe", "Bob"]`,
		`expect body.items.#(id==3) !exists`,
		"expect body.items[0].name == Jane Doe",
		"expect body.items[1].tags.# == 0",
		"expect body.matrix[0][1] == 2",
		"expect body.matrix[1] == [3]",
		"expect body.items[*].id == [1, 2]",
		"expect body.items.#.name length 2",
	)
}

func TestExpectedValueGrammarAtRuntime(t *testing.T) {
	r := resp("application/json", `{"name":"Jane Doe","path":"./x","re":"^x","d":"$x","at":"@x","hash":"#fff","msg":"line\nbreak","num":"42"}`)
	expectPass(t, r,
		"expect body.name == Jane Doe",
		"expect body.path == ./x",
		"expect body.re == ^x",
		"expect body.d == $x",
		"expect body.at == @x",
		"expect body.hash == #fff",
		`expect body.msg == "line\nbreak"`,
		`expect body.num == 42`,
		`expect body.num == "42"`,
		`expect body.name != "Jane"`,
		`expect body.name contains Doe`,
	)
}

func TestHeaderAndStatusSubjectsStillWork(t *testing.T) {
	r := createResponse(201, `{"headers":{"x":1},"bodyText":"t","status":{"code":5}}`, map[string]string{"X-Req": "abc", "Content-Type": "application/json"})
	expectPass(t, r,
		"expect status 201",
		"expect status in [200, 201]",
		"expect status !in [400, 404]",
		"expect header X-Req == abc",
		"expect header x-req exists",
		"expect header X-Nope !exists",
		"expect header Content-Type contains json",
		// body fields that merely start with a keyword
		"expect headers.x == 1",
		"expect bodyText == t",
		"expect status.code == 5",
	)
}

func TestRegexLiteralFlagsAndEscapes(t *testing.T) {
	r := resp("application/json", `{"s":"Hello/World","p":"a.b"}`)
	expectPass(t, r,
		`expect body.s matches /^hello\/world$/i`,
		`expect body.s matches "^Hello/"`,
		`expect body.s matches ^Hello`,
		`expect body.p matches /^a\.b$/`,
	)
	expectFail(t, r, `expect body.p matches /^a\.c$/`, `expect body.s matches /^hello/`)
}
