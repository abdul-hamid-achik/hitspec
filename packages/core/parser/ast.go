package parser

type File struct {
	Path      string
	Variables []*Variable
	Requests  []*Request
}

type Variable struct {
	Name  string
	Value string
	Line  int
}

type Request struct {
	Name         string
	Description  string
	Tags         []string
	Method       string
	URL          string
	Headers      []*Header
	QueryParams  []*QueryParam
	Body         *Body
	Assertions   []*Assertion
	DBAssertions []*DBAssertion
	Captures     []*Capture
	Metadata     *RequestMetadata
	Line         int
}

type DBAssertion struct {
	Query    string
	Column   string
	Operator AssertionOperator
	Expected interface{}
	Line     int
}

type RequestMetadata struct {
	Skip         string
	Only         bool
	Timeout      int
	Retry        int
	RetryDelay   int
	RetryOn      []int
	Depends      []string
	Auth         *AuthConfig
	Condition    *Condition
	PreHooks     []*Hook
	PostHooks    []*Hook
	DBConnection string
}

type AuthConfig struct {
	Type   AuthType
	Params []string
}

type AuthType int

const (
	AuthNone AuthType = iota
	AuthBasic
	AuthBearer
	AuthAPIKey
	AuthAPIKeyQuery
	AuthDigest
	AuthAWS
)

type Condition struct {
	Type       ConditionType
	Expression string
}

type ConditionType int

const (
	ConditionIf ConditionType = iota
	ConditionUnless
)

type Hook struct {
	Type    HookType
	Command string
	Always  bool
}

type HookType int

const (
	HookSet HookType = iota
	HookLog
	HookAssert
	HookExec
)

type Header struct {
	Key   string
	Value string
	Line  int
}

type QueryParam struct {
	Key   string
	Value string
	Line  int
}

type Body struct {
	ContentType BodyType
	Raw         string
	Multipart   []*MultipartField
	GraphQL     *GraphQLBody
	Line        int
}

type BodyType int

const (
	BodyNone BodyType = iota
	BodyJSON
	BodyForm
	BodyFormBlock
	BodyMultipart
	BodyRaw
	BodyXML
	BodyGraphQL
)

type MultipartField struct {
	Type  MultipartFieldType
	Name  string
	Value string
	Path  string
}

type MultipartFieldType int

const (
	MultipartFieldValue MultipartFieldType = iota
	MultipartFieldFile
)

type GraphQLBody struct {
	Query     string
	Variables string
}

type Assertion struct {
	Subject  string
	Operator AssertionOperator
	Expected interface{}
	Line     int
}

type AssertionOperator int

const (
	OpEquals AssertionOperator = iota
	OpNotEquals
	OpGreaterThan
	OpGreaterOrEqual
	OpLessThan
	OpLessOrEqual
	OpContains
	OpNotContains
	OpStartsWith
	OpEndsWith
	OpMatches
	OpExists
	OpNotExists
	OpLength
	OpIncludes
	OpNotIncludes
	OpIn
	OpNotIn
	OpType
	OpEach
	OpSchema
)

func (op AssertionOperator) String() string {
	switch op {
	case OpEquals:
		return "=="
	case OpNotEquals:
		return "!="
	case OpGreaterThan:
		return ">"
	case OpGreaterOrEqual:
		return ">="
	case OpLessThan:
		return "<"
	case OpLessOrEqual:
		return "<="
	case OpContains:
		return "contains"
	case OpNotContains:
		return "!contains"
	case OpStartsWith:
		return "startsWith"
	case OpEndsWith:
		return "endsWith"
	case OpMatches:
		return "matches"
	case OpExists:
		return "exists"
	case OpNotExists:
		return "!exists"
	case OpLength:
		return "length"
	case OpIncludes:
		return "includes"
	case OpNotIncludes:
		return "!includes"
	case OpIn:
		return "in"
	case OpNotIn:
		return "!in"
	case OpType:
		return "type"
	case OpEach:
		return "each"
	case OpSchema:
		return "schema"
	default:
		return "unknown"
	}
}

type Capture struct {
	Name   string
	Source CaptureSource
	Path   string
	Line   int
}

type CaptureSource int

const (
	CaptureBody CaptureSource = iota
	CaptureHeader
	CaptureStatus
	CaptureDuration
)

func (s CaptureSource) String() string {
	switch s {
	case CaptureBody:
		return "body"
	case CaptureHeader:
		return "header"
	case CaptureStatus:
		return "status"
	case CaptureDuration:
		return "duration"
	default:
		return "unknown"
	}
}

type Position struct {
	Line   int
	Column int
}

type ParseError struct {
	File    string
	Line    int
	Column  int
	Message string
	Snippet string
}

func (e *ParseError) Error() string {
	if e.File != "" {
		return e.File + ":" + itoa(e.Line) + ":" + itoa(e.Column) + ": " + e.Message
	}
	return "line " + itoa(e.Line) + ": " + e.Message
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
