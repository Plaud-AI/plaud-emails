package dao

// IDConstraint is a generic constraint that permits int64 and string
// Use it to write functions that accept either numeric or string IDs.
type IDConstraint interface {
	~int64 | ~string
}
