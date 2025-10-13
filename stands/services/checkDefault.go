package services

func IsDefaultStandsFilter(queryFilters map[string]string, dynamicFields map[string]bool) bool {

	// Check if 'status' exists
	if _, ok := queryFilters["status"]; !ok {
		return false
	}

	// Check for any additional filters
	for key := range queryFilters {
		if _, exists := dynamicFields[key]; !exists || !dynamicFields[key] {
			return false
		}
	}

	return true
}
