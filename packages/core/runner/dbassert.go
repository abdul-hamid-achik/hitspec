package runner

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/db"
)

// DBAssertionResult represents the result of a database assertion
type DBAssertionResult struct {
	Query    string
	Column   string
	Passed   bool
	Expected interface{}
	Actual   interface{}
	Message  string
}

// executeDBAssertions runs all database assertions for a request
func (r *Runner) executeDBAssertions(dbAssertions []*parser.DBAssertion, connStr string, resolver func(string) string) ([]*DBAssertionResult, error) {
	if len(dbAssertions) == 0 {
		return nil, nil
	}

	// Resolve variables in connection string
	connStr = resolver(connStr)

	// Connect to database
	client, err := db.NewClient(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer client.Close()

	var results []*DBAssertionResult

	// Group assertions by query to avoid running the same query multiple times
	queryGroups := make(map[string][]*parser.DBAssertion)
	queryOrder := make([]string, 0)

	for _, assertion := range dbAssertions {
		query := resolver(assertion.Query)
		if _, exists := queryGroups[query]; !exists {
			queryOrder = append(queryOrder, query)
		}
		queryGroups[query] = append(queryGroups[query], assertion)
	}

	// Execute each query and evaluate assertions
	for _, query := range queryOrder {
		queryAssertions := queryGroups[query]

		queryResult, err := client.Query(query)
		if err != nil {
			// Create failed results for all assertions on this query
			for _, assertion := range queryAssertions {
				results = append(results, &DBAssertionResult{
					Query:   query,
					Column:  assertion.Column,
					Passed:  false,
					Message: fmt.Sprintf("query failed: %v", err),
				})
			}
			continue
		}

		// Evaluate each assertion
		for _, assertion := range queryAssertions {
			result := evaluateDBAssertion(assertion, queryResult, resolver)
			result.Query = query
			results = append(results, result)
		}
	}

	return results, nil
}

// evaluateDBAssertion evaluates a single database assertion
func evaluateDBAssertion(assertion *parser.DBAssertion, result *db.QueryResult, resolver func(string) string) *DBAssertionResult {
	res := &DBAssertionResult{
		Column:   assertion.Column,
		Expected: assertion.Expected,
		Passed:   false,
	}

	if len(result.Rows) == 0 {
		res.Message = "query returned no rows"
		return res
	}

	// Get the first row (most common case)
	row := result.Rows[0]

	// Get the column value
	actual, exists := row[assertion.Column]
	if !exists {
		// Try case-insensitive match
		for col, val := range row {
			if equalFold(col, assertion.Column) {
				actual = val
				exists = true
				break
			}
		}
	}

	if !exists {
		res.Message = fmt.Sprintf("column %q not found in result", assertion.Column)
		return res
	}

	res.Actual = actual

	// Resolve expected value if it's a string with variables
	expected := assertion.Expected
	if s, ok := expected.(string); ok {
		expected = resolver(s)
	}

	// Compare values based on operator
	passed, msg := compareDBValues(actual, assertion.Operator, expected)
	res.Passed = passed
	if !passed {
		res.Message = msg
	}

	return res
}

// compareDBValues compares two values using the given operator
func compareDBValues(actual interface{}, op parser.AssertionOperator, expected interface{}) (bool, string) {
	switch op {
	case parser.OpEquals:
		return dbAssertEquals(actual, expected)
	case parser.OpNotEquals:
		passed, _ := dbAssertEquals(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to equal %v", expected)
		}
		return true, ""
	case parser.OpGreaterThan:
		return dbAssertCompareNumeric(actual, expected, ">")
	case parser.OpGreaterOrEqual:
		return dbAssertCompareNumeric(actual, expected, ">=")
	case parser.OpLessThan:
		return dbAssertCompareNumeric(actual, expected, "<")
	case parser.OpLessOrEqual:
		return dbAssertCompareNumeric(actual, expected, "<=")
	case parser.OpContains:
		return dbAssertContains(actual, expected)
	case parser.OpExists:
		if actual == nil {
			return false, "expected to exist"
		}
		return true, ""
	case parser.OpNotExists:
		if actual != nil {
			return false, "expected not to exist"
		}
		return true, ""
	default:
		return false, fmt.Sprintf("unsupported operator for db assertion: %s", op.String())
	}
}

// dbAssertEquals checks equality between two values
func dbAssertEquals(actual, expected interface{}) (bool, string) {
	if reflect.DeepEqual(actual, expected) {
		return true, ""
	}

	// Try numeric comparison
	actualNum, aOk := toFloat64DB(actual)
	expectedNum, eOk := toFloat64DB(expected)
	if aOk && eOk && actualNum == expectedNum {
		return true, ""
	}

	// Try string comparison
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	if actualStr == expectedStr {
		return true, ""
	}

	return false, fmt.Sprintf("expected %v, got %v", expected, actual)
}

// dbAssertCompareNumeric compares two values numerically
func dbAssertCompareNumeric(actual, expected interface{}, op string) (bool, string) {
	actualNum, aOk := toFloat64DB(actual)
	expectedNum, eOk := toFloat64DB(expected)

	if !aOk || !eOk {
		return false, fmt.Sprintf("cannot compare non-numeric values: %v %s %v", actual, op, expected)
	}

	var passed bool
	switch op {
	case ">":
		passed = actualNum > expectedNum
	case ">=":
		passed = actualNum >= expectedNum
	case "<":
		passed = actualNum < expectedNum
	case "<=":
		passed = actualNum <= expectedNum
	}

	if passed {
		return true, ""
	}
	return false, fmt.Sprintf("expected %v %s %v", actual, op, expected)
}

// dbAssertContains checks if actual contains expected
func dbAssertContains(actual, expected interface{}) (bool, string) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	if strings.Contains(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected '%v' to contain '%v'", actual, expected)
}

// toFloat64DB converts a value to float64
func toFloat64DB(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// equalFold is a simple case-insensitive string comparison
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
