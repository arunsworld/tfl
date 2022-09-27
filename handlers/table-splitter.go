package handlers

func splitIntoTabularFormat[K any](input []K, maxPerRow int) [][]K {
	if len(input) == 0 {
		return [][]K{}
	}
	currentRow := 0
	result := [][]K{
		{},
	}
	for _, l := range input {
		if len(result[currentRow]) == maxPerRow {
			result = append(result, []K{})
			currentRow++
		}
		result[currentRow] = append(result[currentRow], l)
	}
	return result
}
