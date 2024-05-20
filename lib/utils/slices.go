package utils

func Splice(slice []string, start, deleteCount int, items ...string) []string {
	if start < 0 {
		start = len(slice) + start
		if start < 0 {
			start = 0
		}
	}

	if deleteCount < 0 {
		deleteCount = 0
	}

	end := start + deleteCount
	if end > len(slice) {
		end = len(slice)
	}

	result := append(slice[:start], items...)
	result = append(result, slice[end:]...)

	return result
}
