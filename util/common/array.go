package common

func UnionUintArray(a []uint, b []uint) []uint {
	m := make(map[uint]bool)
	for _, v := range a {
		m[v] = true
	}
	for _, v := range b {
		m[v] = true
	}
	var res []uint
	for k := range m {
		res = append(res, k)
	}
	return res
}

// Find different elements in two slices
// Returns elements in 'a' that are not in 'b' and elements in 'b' that are not in 'a'
func DiffUintArray(a []uint, b []uint) []uint {
	different := []uint{}
	set := make(map[uint]bool)

	for _, item := range a {
		set[item] = true
	}
	for _, item := range b {
		if !set[item] {
			different = append(different, item)
		}
	}

	set = make(map[uint]bool)
	for _, item := range b {
		set[item] = true
	}
	for _, item := range a {
		if !set[item] {
			different = append(different, item)
		}
	}

	return different
}
