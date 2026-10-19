package parser

import "fmt"

// ParseSource parses a document that is not a file on disk (stdin) and prints
// its warnings once, like ParseFile. name is used for error and warning
// positions.
func ParseSource(input, name string) (*File, error) {
	f, err := Parse(input, name)
	if err == nil && WarningOutput != nil {
		for _, w := range f.Warnings {
			if _, seen := printedWarnings.LoadOrStore(w.String(), true); seen {
				continue
			}
			fmt.Fprintf(WarningOutput, "warning: %s\n", w)
		}
	}
	return f, err
}
