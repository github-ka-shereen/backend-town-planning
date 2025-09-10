package utils

import "time"

// IsDefaultFilter compares queryFilters against a given defaultFilters map.
// It checks for missing keys, additional keys, or value differences.
func IsDefaultFilter(queryFilters, defaultFilters map[string]string) bool {
	// Check if all keys in defaultFilters exist in queryFilters with the same value
	for key, defaultValue := range defaultFilters {
		if queryFilters[key] != defaultValue {
			return false
		}
	}

	// Check for additional keys in queryFilters that are not in defaultFilters
	for key := range queryFilters {
		if _, exists := defaultFilters[key]; !exists {
			return false
		}
	}

	// If no discrepancies are found, the filters match the default
	return true
}

func IsDefaultPaymentFilter(queryFilters map[string]string, dynamicFields map[string]bool) bool {
	// Get today's date in 'yyyy-mm-dd' format
	today := time.Now().Format("2006-01-02")

	// Check if 'start_date' and 'end_date' are today's date
	if startDate, ok := queryFilters["start_date"]; !ok || startDate != today {
		return false
	}
	if endDate, ok := queryFilters["end_date"]; !ok || endDate != today {
		return false
	}

	// Check if 'payment_method' exists
	if _, ok := queryFilters["payment_method"]; !ok {
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

func IsDefaultFilterForQuery(queryFilters map[string]string, dynamicFields map[string]bool) bool {

	// Check if 'active' exists
	if _, ok := queryFilters["active"]; !ok {
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
