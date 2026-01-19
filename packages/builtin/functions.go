package builtin

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Func func(args []string) any

type Registry struct {
	funcs map[string]Func
}

func NewRegistry() *Registry {
	r := &Registry{
		funcs: make(map[string]Func),
	}
	r.registerDefaults()
	return r
}

func (r *Registry) registerDefaults() {
	r.funcs["now"] = funcNow
	r.funcs["timestamp"] = funcTimestamp
	r.funcs["timestampMs"] = funcTimestampMs
	r.funcs["uuid"] = funcUUID
	r.funcs["random"] = funcRandom
	r.funcs["randomString"] = funcRandomString
	r.funcs["randomEmail"] = funcRandomEmail
	r.funcs["randomAlphanumeric"] = funcRandomAlphanumeric
	r.funcs["base64"] = funcBase64
	r.funcs["base64Decode"] = funcBase64Decode
	r.funcs["md5"] = funcMD5
	r.funcs["sha256"] = funcSHA256
	r.funcs["urlEncode"] = funcURLEncode
	r.funcs["urlDecode"] = funcURLDecode
	r.funcs["date"] = funcDate
	r.funcs["json"] = funcJSON
}

func (r *Registry) Register(name string, fn Func) {
	r.funcs[name] = fn
}

var funcCallPattern = regexp.MustCompile(`^(\w+)\((.*)\)$`)

func (r *Registry) Call(expr string) (any, bool) {
	matches := funcCallPattern.FindStringSubmatch(expr)
	if matches == nil {
		return nil, false
	}

	name := matches[1]
	argsStr := matches[2]

	fn, ok := r.funcs[name]
	if !ok {
		return nil, false
	}

	var args []string
	if argsStr != "" {
		args = parseArgs(argsStr)
	}

	return fn(args), true
}

func parseArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !inQuote && (ch == '"' || ch == '\'') {
			inQuote = true
			quoteChar = ch
		} else if inQuote && ch == quoteChar {
			inQuote = false
			quoteChar = 0
		} else if !inQuote && ch == ',' {
			args = append(args, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	return args
}

func funcNow(_ []string) any {
	return time.Now().UTC().Format(time.RFC3339)
}

func funcTimestamp(_ []string) any {
	return time.Now().Unix()
}

func funcTimestampMs(_ []string) any {
	return time.Now().UnixMilli()
}

func funcUUID(_ []string) any {
	return uuid.New().String()
}

func funcRandom(args []string) any {
	min, max := 0, 100
	if len(args) >= 2 {
		if v, err := strconv.Atoi(args[0]); err == nil {
			min = v
		} else {
			fmt.Fprintf(os.Stderr, "warning: random() min argument %q is not a valid integer\n", args[0])
		}
		if v, err := strconv.Atoi(args[1]); err == nil {
			max = v
		} else {
			fmt.Fprintf(os.Stderr, "warning: random() max argument %q is not a valid integer\n", args[1])
		}
	}
	return rand.Intn(max-min+1) + min
}

func funcRandomString(args []string) any {
	length := 16
	if len(args) >= 1 {
		if v, err := strconv.Atoi(args[0]); err == nil {
			length = v
		} else {
			fmt.Fprintf(os.Stderr, "warning: randomString() length argument %q is not a valid integer\n", args[0])
		}
	}
	return randomString(length, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

func funcRandomEmail(_ []string) any {
	user := randomString(8, "abcdefghijklmnopqrstuvwxyz")
	domain := randomString(6, "abcdefghijklmnopqrstuvwxyz")
	return fmt.Sprintf("%s@%s.com", user, domain)
}

func funcRandomAlphanumeric(args []string) any {
	length := 8
	if len(args) >= 1 {
		if v, err := strconv.Atoi(args[0]); err == nil {
			length = v
		} else {
			fmt.Fprintf(os.Stderr, "warning: randomAlphanumeric() length argument %q is not a valid integer\n", args[0])
		}
	}
	return randomString(length, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

func funcBase64(args []string) any {
	if len(args) < 1 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(args[0]))
}

func funcBase64Decode(args []string) any {
	if len(args) < 1 {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		return ""
	}
	return string(decoded)
}

func funcMD5(args []string) any {
	if len(args) < 1 {
		return ""
	}
	hash := md5.Sum([]byte(args[0]))
	return hex.EncodeToString(hash[:])
}

func funcSHA256(args []string) any {
	if len(args) < 1 {
		return ""
	}
	hash := sha256.Sum256([]byte(args[0]))
	return hex.EncodeToString(hash[:])
}

func funcURLEncode(args []string) any {
	if len(args) < 1 {
		return ""
	}
	return url.QueryEscape(args[0])
}

func funcURLDecode(args []string) any {
	if len(args) < 1 {
		return ""
	}
	decoded, err := url.QueryUnescape(args[0])
	if err != nil {
		return args[0]
	}
	return decoded
}

func funcDate(args []string) any {
	format := "2006-01-02"
	if len(args) >= 1 {
		format = args[0]
	}
	return time.Now().UTC().Format(format)
}

func funcJSON(args []string) any {
	if len(args) < 1 {
		return ""
	}
	return args[0]
}

func randomString(length int, charset string) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
