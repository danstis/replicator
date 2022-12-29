package utility

// ValueDefault will return the value (v) if it has a value, otherwise it will return the default (d).
func ValueDefault(v, d string) string {
	if len(v) == 0 {
		return d
	}
	return v
}
