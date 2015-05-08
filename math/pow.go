package math

func Pow(a uint64, b int) uint64 {
	var result uint64 = 1

	for 0 != b {
		if 0 != (b & 1) {
			result *= uint64(a)

		}
		b >>= 1
		a *= a
	}

	return result
}
