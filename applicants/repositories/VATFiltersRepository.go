package repositories

import (
	"strings"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type vatQueryBuilder struct {
	db      *gorm.DB
	filters map[string]string
}

func newVatQueryBuilder(db *gorm.DB, filters map[string]string) *vatQueryBuilder {
	return &vatQueryBuilder{db: db, filters: filters}
}

func (qb *vatQueryBuilder) applyBasicFilters() *vatQueryBuilder {
	db := qb.db.Model(&models.VATRate{})
	if active, ok := qb.filters["active"]; ok && active != "" {
		db = db.Where("active = ?", strings.ToLower(active) == "true")
	}
	if used, ok := qb.filters["used"]; ok && used != "" {
		db = db.Where("used = ?", strings.ToLower(used) == "true")
	}

	qb.db = db
	return qb
}

func (qb *vatQueryBuilder) applyDateRangeFilter() (*gorm.DB, error) {
	db := qb.db
	startDateStr := qb.filters["start_date"]
	endDateStr := qb.filters["end_date"]

	var startDate, endDate time.Time
	var err error

	// Parse startDate if provided
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return nil, err
		}
	}

	// Parse and adjust endDate if provided
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return nil, err
		}
		endDate = endDate.Add(24*time.Hour - time.Second)
	}

	// Apply date range filters
	if startDateStr != "" && endDateStr != "" {
		db = db.Where("valid_from <= ?", endDate).
			Where("(valid_to >= ? OR valid_to IS NULL)", startDate)
	} else if startDateStr != "" {
		db = db.Where("(valid_to >= ? OR valid_to IS NULL)", startDate)
	} else if endDateStr != "" {
		db = db.Where("valid_from <= ?", endDate)
	}

	return db, nil
}

// checkExactMatch checks if there's an exact date match for VAT rates
func (qb *vatQueryBuilder) checkExactMatch(startDate string) bool {
	var exactMatch models.VATRate
	query := qb.db.Session(&gorm.Session{}).
		Where("DATE(valid_from) = ?", startDate) // Use DATE() for date comparison

	return query.First(&exactMatch).Error == nil
}

// findClosestRecord finds the closest older record for a given date and tax type
func (qb *vatQueryBuilder) findClosestRecord(startDate string) (*models.VATRate, bool) {
	var closestRecord models.VATRate
	parsedDate, err := time.Parse("2006-01-02", startDate) // Assuming YYYY-MM-DD format
	if err != nil {
		config.Logger.Error("Failed to parse start_date for closest record", zap.Error(err), zap.String("startDate", startDate))
		return nil, false
	}

	closestQuery := qb.db.Session(&gorm.Session{}).
		Where("valid_from < ?", parsedDate). // Look for records strictly older than startDate
		Order("valid_from DESC")             // Get the newest among the older ones

	result := closestQuery.First(&closestRecord)
	if result.Error != nil {
		if result.Error != gorm.ErrRecordNotFound {
			config.Logger.Error("Failed to find closest VAT record", zap.Error(result.Error), zap.String("startDate", startDate))
		}
		return nil, false
	}
	return &closestRecord, true
}

// GetFilteredVatRates fetches paginated VAT rates based on filters
func (r *applicantRepository) GetFilteredVatRates(limit, offset int, filters map[string]string) ([]models.VATRate, int64, error) { // Changed to models.VATRate
	return r.getVatRates(filters, true, limit, offset)
}

// getVatRates handles the common logic for retrieving VAT rates
func (r *applicantRepository) getVatRates(filters map[string]string, paginationEnabled bool, limit, offset int) ([]models.VATRate, int64, error) { // Changed to models.VATRate
	qb := newVatQueryBuilder(r.DB, filters).applyBasicFilters()

	rangeQuery, err := qb.applyDateRangeFilter()
	if err != nil {
		return nil, 0, err
	}

	var vatRates []models.VATRate // Changed to models.VATRate
	startDate := filters["start_date"]

	// This logic is for finding a "closest record" if the start_date implies
	// a point-in-time query rather than a strict range.
	// The condition has been updated to use the actual checkExactMatch logic
	if startDate != "" && !qb.checkExactMatch(startDate) {
		if closestRecord, exists := qb.findClosestRecord(startDate); exists {
			if err := rangeQuery.Find(&vatRates).Error; err != nil {
				return nil, 0, err
			}

			// Add closest record if not already in results
			found := false
			for _, rate := range vatRates {
				if rate.ID == closestRecord.ID {
					found = true
					break
				}
			}
			if !found {
				// Dereference closestRecord here
				vatRates = append([]models.VATRate{*closestRecord}, vatRates...)
			}
		}
	} else {
		// If no specific start date or an exact match is expected, just find
		if err := rangeQuery.Find(&vatRates).Error; err != nil {
			return nil, 0, err
		}
	}

	total := int64(len(vatRates)) // Total records before pagination

	if paginationEnabled {
		// Apply pagination by slicing the results
		start := offset
		end := offset + limit
		if start > len(vatRates) {
			start = len(vatRates)
		}
		if end > len(vatRates) {
			end = len(vatRates)
		}
		vatRates = vatRates[start:end]
	}

	return vatRates, total, nil
}
