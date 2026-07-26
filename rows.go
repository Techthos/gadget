package gomukit

import (
	"encoding/json"
	"fmt"
)

// RowsOf converts a typed slice into row maps via encoding/json, honoring
// json struct tags. Use it to feed typed application data into
// Table.InitialData or a tool result's structuredContent:
//
//	rows, _ := gomukit.RowsOf(users)
//	table.InitialData = map[string]any{"rows": rows}
func RowsOf(slice any) ([]map[string]any, error) {
	b, err := json.Marshal(slice)
	if err != nil {
		return nil, fmt.Errorf("gomukit: RowsOf: %w", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("gomukit: RowsOf: value is not a slice of objects: %w", err)
	}
	return rows, nil
}
