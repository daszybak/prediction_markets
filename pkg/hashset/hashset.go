package hashset

func NewSet[T comparable]() Set[T] {
	return map[T]struct{}{}
}

type Set[T comparable] map[T]struct{}

func SetFromSlice[T comparable](vals []T) Set[T] {
	set := NewSet[T]()
	for _, vals := range vals {
		set.Set(vals)
	}
	return set
}

func (vs Set[T]) Set(v T) {
	vs[v] = struct{}{}
}

func (vs Set[T]) Has(v T) bool {
	_, ok := vs[v]
	return ok
}

func (vs Set[T]) HasAny(xs Set[T]) bool {
	for x := range xs {
		if vs.Has(x) {
			return true
		}
	}
	return false
}

func (vs Set[T]) Remove(xs Set[T]) Set[T] {
	result := NewSet[T]()
	for v := range vs {
		if !xs.Has(v) {
			result.Set(v)
		}
	}
	return result
}

func (vs Set[T]) AsSlice() []T {
	slice := make([]T, 0, len(vs))
	for s := range vs {
		slice = append(slice, s)
	}
	return slice
}
