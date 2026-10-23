package coinmarketcap

// Optional params fields are pointers. These helpers return a pointer to
// their argument.

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// String returns a pointer to s.
func String(s string) *string { return &s }

// Int returns a pointer to i.
func Int(i int) *int { return &i }

// Int64 returns a pointer to i.
func Int64(i int64) *int64 { return &i }

// Float64 returns a pointer to f.
func Float64(f float64) *float64 { return &f }

// Bool returns a pointer to b.
func Bool(b bool) *bool { return &b }
