package helpers

func NormalizeLimit(value int) int {
	if value <= 0 || value > 1000 {
		return 100
	}
	return value
}
