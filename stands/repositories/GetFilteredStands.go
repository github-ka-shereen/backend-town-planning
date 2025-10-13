package repositories

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/stands/services"

	"go.uber.org/zap"
	"gorm.io/gorm"
)



// standsQueryBuilder builds queries for stand filtering
type standsQueryBuilder struct {
	query   *gorm.DB
	filters map[string]string
}

func newStandsQueryBuilder(db *gorm.DB, filters map[string]string) *standsQueryBuilder {
	return &standsQueryBuilder{
		query:   db.Model(&models.Stand{}),
		filters: filters,
	}
}

func (pqb *standsQueryBuilder) applyBasicStandsFilters() *standsQueryBuilder {
	if status, ok := pqb.filters["status"]; ok {
		pqb.query = pqb.query.Where("status = ?", status)
	}
	if projectNumber, ok := pqb.filters["project_id"]; ok {
		pqb.query = pqb.query.Where("project_id = ?", projectNumber)
	}
	if standCurrency, ok := pqb.filters["stand_currency"]; ok {
		pqb.query = pqb.query.Where("stand_currency = ?", standCurrency)
	}
	return pqb
}

func (pqb *standsQueryBuilder) applyStandsDateRangeFilter() *standsQueryBuilder {
	startDate := pqb.filters["start_date"]
	endDate := pqb.filters["end_date"]

	if startDate != "" && startDate != "null" && endDate != "" && endDate != "null" {
		pqb.query = pqb.query.Where("DATE(created_at) BETWEEN DATE(?) AND DATE(?)", startDate, endDate)
	}
	return pqb
}

func (pqb *standsQueryBuilder) applyLatestOrder() *standsQueryBuilder {
	pqb.query = pqb.query.Order("GREATEST(created_at, updated_at) DESC").Order("created_at DESC")
	return pqb
}

func (pqb *standsQueryBuilder) Limit(limit int) *standsQueryBuilder {
	pqb.query = pqb.query.Limit(limit)
	return pqb
}

func (pqb *standsQueryBuilder) Offset(offset int) *standsQueryBuilder {
	pqb.query = pqb.query.Offset(offset)
	return pqb
}

// GetFilteredStands returns filtered stands with pagination
func (r *standRepository) GetFilteredStands(filters map[string]string, paginationEnabled bool, limit, offset int) ([]models.Stand, int64, error) {
	pqb := newStandsQueryBuilder(r.db, filters).applyBasicStandsFilters().applyStandsDateRangeFilter().applyLatestOrder()
	pqb2 := newStandsQueryBuilder(r.db, filters).applyBasicStandsFilters().applyStandsDateRangeFilter()

	if paginationEnabled {
		pqb = pqb.Limit(limit).Offset(offset)
	}

	var stands []models.Stand
	if err := pqb.query.Preload("Project").Preload("CurrentOwner").Preload("PreviousOwner").Preload("AllStandOwners").Preload("AllStandOwners.Applicant").Find(&stands).Error; err != nil {
		return nil, 0, err
	}

	var total int64
	if err := pqb2.query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return stands, total, nil
}

// GetFilteredAllStandsResults handles stand filtering with background processing for long-running queries
func (r *standRepository) GetFilteredAllStandsResults(filters map[string]string, userEmail string) ([]models.Stand, int64, bool, error) {
	startTime := time.Now()

	stands, total, err := r.GetFilteredStands(filters, false, 0, 0)
	if err != nil {
		return nil, 0, false, err
	}

	if time.Since(startTime) > 5*time.Second {
		go r.processStandsBackgroundTask(filters, userEmail)
		return nil, 0, true, nil
	}

	return stands, total, false, nil
}

// Background processing for long-running stand queries
func (r *standRepository) processStandsBackgroundTask(filters map[string]string, userEmail string) {
	filePrefixName := services.ToCamelCase(filters["status"]) + "_" + "Stands_Report"

	_, err := services.StartBackgroundProcess(
		filePrefixName,
		filters,
		userEmail,
		r.BackgroundStandsTaskFunction,
		[]string{"StandNumber", "ProjectNumber", "PaymentFor", "ClientName", "StandNumber", "StandCurrency", "StandCost", "StandType", "Active", "CreatedAt", "CreatedBy"},
		"Payments Report",
		"Your payment report has been processed successfully.",
	)
	if err != nil {
		config.Logger.Error("Failed to offload background task", zap.Error(err))
	}
}

// BackgroundStandsTaskFunction handles background execution for stand reports
func (r *standRepository) BackgroundStandsTaskFunction(filters map[string]string) ([]interface{}, error) {
	stands, _, err := r.GetFilteredStands(filters, false, 0, 0)
	if err != nil {
		return nil, err
	}

	resultData := make([]interface{}, len(stands))
	for i, stand := range stands {
		resultData[i] = stand
	}

	return resultData, nil
}

// Reserved stands query builder
type reservedStandsQueryBuilder struct {
	query   *gorm.DB
	filters map[string]string
}

func newReservedStandsQueryBuilder(db *gorm.DB, filters map[string]string) *reservedStandsQueryBuilder {
	return &reservedStandsQueryBuilder{
		query:   db.Model(&models.Reservation{}),
		filters: filters,
	}
}

func (rq *reservedStandsQueryBuilder) applyBasicReservedFilters() *reservedStandsQueryBuilder {
	if status, ok := rq.filters["status"]; ok {
		rq.query = rq.query.Where("status = ?", status)
	}
	return rq
}

func (rq *reservedStandsQueryBuilder) applyReservedDateRangeFilter() *reservedStandsQueryBuilder {
	startDate := rq.filters["start_date"]
	endDate := rq.filters["end_date"]

	if startDate != "" && startDate != "null" && endDate != "" && endDate != "null" {
		rq.query = rq.query.Where("DATE(created_at) BETWEEN DATE(?) AND DATE(?)", startDate, endDate)
	}
	return rq
}

func (rq *reservedStandsQueryBuilder) applyReservedOrder() *reservedStandsQueryBuilder {
	rq.query = rq.query.Order("created_at DESC")
	return rq
}

func (rq *reservedStandsQueryBuilder) Limit(limit int) *reservedStandsQueryBuilder {
	rq.query = rq.query.Limit(limit)
	return rq
}

func (rq *reservedStandsQueryBuilder) Offset(offset int) *reservedStandsQueryBuilder {
	rq.query = rq.query.Offset(offset)
	return rq
}

// GetFilteredReservedStands returns filtered reserved stands
func (r *standRepository) GetFilteredReservedStands(filters map[string]string, paginationEnabled bool, limit, offset int) ([]models.Reservation, int64, error) {
	rqb := newReservedStandsQueryBuilder(r.db, filters).applyBasicReservedFilters().applyReservedDateRangeFilter().applyReservedOrder()
	rqb2 := newReservedStandsQueryBuilder(r.db, filters).applyBasicReservedFilters().applyReservedDateRangeFilter()

	if paginationEnabled {
		rqb = rqb.Limit(limit).Offset(offset)
	}

	var reservations []models.Reservation
	if err := rqb.query.Preload("Stand").Preload("Client").Preload("ReservationOwners.Client").Find(&reservations).Error; err != nil {
		return nil, 0, err
	}

	var total int64
	if err := rqb2.query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return reservations, total, nil
}

// GetFilteredAllFilteredReservedStandsResults handles reserved stand filtering with background processing
func (r *standRepository) GetFilteredAllFilteredReservedStandsResults(filters map[string]string, userEmail string) ([]models.Reservation, int64, bool, error) {
	startTime := time.Now()

	reservations, total, err := r.GetFilteredReservedStands(filters, false, 0, 0)
	if err != nil {
		return nil, 0, false, err
	}

	if time.Since(startTime) > 5*time.Second {
		go r.processReservedStandsBackgroundTask(filters, userEmail)
		return nil, 0, true, nil
	}

	return reservations, total, false, nil
}

func (r *standRepository) processReservedStandsBackgroundTask(filters map[string]string, userEmail string) {
	filePrefixName := services.ToCamelCase(filters["status"]) + "_" + "Reserved_Stands_Report"

	_, err := services.StartBackgroundProcess(
		filePrefixName,
		filters,
		userEmail,
		r.BackgroundReservedStandsTaskFunction,
		[]string{"StandNumber", "ClientName", "Status", "ExpiryDate", "CreatedAt"},
		"Reserved Stands Report",
		"Your reserved stands report has been processed successfully.",
	)
	if err != nil {
		config.Logger.Error("Failed to offload background task", zap.Error(err))
	}
}

func (r *standRepository) BackgroundReservedStandsTaskFunction(filters map[string]string) ([]interface{}, error) {
	reservations, _, err := r.GetFilteredReservedStands(filters, false, 0, 0)
	if err != nil {
		return nil, err
	}

	resultData := make([]interface{}, len(reservations))
	for i, reservation := range reservations {
		resultData[i] = reservation
	}

	return resultData, nil
}
