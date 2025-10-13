package repositories

import (
	"strings"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (r *BleveRepository) IndexSingleStand(stand models.Stand) error {
	// Determine the owner name for indexing
	var ownerName string
	if stand.CurrentOwner != nil && stand.CurrentOwner.FullName != "" {
		ownerName = stand.CurrentOwner.FullName
	} else if stand.CurrentOwnerID == nil {
		ownerName = "Unallocated"
	}

	// Define the minimal document structure for Bleve indexing
	bleveStandDoc := struct {
		ID             string               `json:"id"`
		StandNumber    string               `json:"stand_number"`
		ProjectID      string               `json:"project_id,omitempty"`
		CurrentOwnerID string               `json:"current_owner_id,omitempty"`
		OwnerName      string               `json:"owner_name"`
		Status         string               `json:"status"`
		StandType      string               `json:"stand_type,omitempty"`
		StandCurrency  models.StandCurrency `json:"stand_currency"`
		IsActive       bool                 `json:"is_active"`
	}{
		ID:             stand.ID.String(),
		StandNumber:    stand.StandNumber,
		ProjectID:      derefUUID(stand.ProjectID),
		CurrentOwnerID: derefUUID(stand.CurrentOwnerID),
		OwnerName:      ownerName,
		Status:         string(stand.Status),
		StandType:      getStandTypeName(stand.StandType),
		StandCurrency:  stand.StandCurrency,
		IsActive:       stand.IsActive,
	}

	// Index the document into Bleve
	err := r.indexer.IndexDocument("stands", stand.ID.String(), bleveStandDoc)
	if err != nil {
		config.Logger.Error("Failed to index single stand into Bleve",
			zap.Error(err),
			zap.String("stand_id", stand.ID.String()))
		return err
	}

	config.Logger.Info("Successfully indexed single stand into Bleve",
		zap.String("stand_id", stand.ID.String()))
	return nil
}

func (r *BleveRepository) IndexExistingStands(stands []models.Stand) error {
	docsToBleveIndex := make(map[string]interface{})

	for _, stand := range stands {
		// Determine the owner name for indexing
		var ownerName string
		if stand.CurrentOwner != nil && stand.CurrentOwner.FullName != "" {
			ownerName = stand.CurrentOwner.FullName
		} else if stand.CurrentOwnerID == nil {
			ownerName = "Unallocated"
		}

		bleveStandDoc := struct {
			ID             string               `json:"id"`
			StandNumber    string               `json:"stand_number"`
			ProjectID      string               `json:"project_id,omitempty"`
			CurrentOwnerID string               `json:"current_owner_id,omitempty"`
			OwnerName      string               `json:"owner_name"`
			Status         string               `json:"status"`
			StandType      string               `json:"stand_type,omitempty"`
			StandCurrency  models.StandCurrency `json:"stand_currency"`
			IsActive       bool                 `json:"is_active"`
		}{
			ID:             stand.ID.String(),
			StandNumber:    stand.StandNumber,
			ProjectID:      derefUUID(stand.ProjectID),
			CurrentOwnerID: derefUUID(stand.CurrentOwnerID),
			OwnerName:      ownerName,
			Status:         string(stand.Status),
			StandType:      getStandTypeName(stand.StandType),
			StandCurrency:  stand.StandCurrency,
			IsActive:       stand.IsActive,
		}

		docsToBleveIndex[stand.ID.String()] = bleveStandDoc
	}

	if len(docsToBleveIndex) > 0 {
		config.Logger.Info("Attempting to bulk index stands into Bleve",
			zap.Int("count", len(docsToBleveIndex)))
		err := r.indexer.BulkIndexDocuments("stands", docsToBleveIndex)
		if err != nil {
			config.Logger.Error("Failed to bulk index stands into Bleve", zap.Error(err))
			return err
		}
		config.Logger.Info("Successfully bulk indexed stands into Bleve",
			zap.Int("count", len(docsToBleveIndex)))
	} else {
		config.Logger.Info("No stands to index into Bleve.")
	}

	return nil
}

func (r *BleveRepository) SearchStands(
	queryString string,
	status string,
	standType string,
	active *bool,
	standCurrency string,
) (*bleve.SearchResult, error) {
	booleanQuery := bleve.NewBooleanQuery()
	queryString = strings.TrimSpace(queryString)
	queryStringLower := strings.ToLower(queryString)

	// Search strategies
	if queryString != "" {
		exactMatch := bleve.NewBooleanQuery()

		// Exact matches for stand number
		standNumExact := bleve.NewTermQuery(queryString)
		standNumExact.SetField("stand_number")
		standNumExact.SetBoost(10.0)
		exactMatch.AddShould(standNumExact)

		standNumExactLower := bleve.NewTermQuery(queryStringLower)
		standNumExactLower.SetField("stand_number")
		standNumExactLower.SetBoost(9.0)
		exactMatch.AddShould(standNumExactLower)

		// Owner name exact matches
		ownerNameExact := bleve.NewTermQuery(queryStringLower)
		ownerNameExact.SetField("owner_name")
		ownerNameExact.SetBoost(8.0)
		exactMatch.AddShould(ownerNameExact)

		// Match query for analyzed fields
		matchQuery := bleve.NewMatchQuery(queryString)
		matchQuery.SetField("stand_number")
		matchQuery.SetBoost(7.0)
		exactMatch.AddShould(matchQuery)

		// Prefix matches
		prefixMatch := bleve.NewBooleanQuery()

		standNumPrefix := bleve.NewPrefixQuery(queryStringLower)
		standNumPrefix.SetField("stand_number")
		standNumPrefix.SetBoost(6.0)
		prefixMatch.AddShould(standNumPrefix)

		ownerNamePrefix := bleve.NewPrefixQuery(queryStringLower)
		ownerNamePrefix.SetField("owner_name")
		ownerNamePrefix.SetBoost(5.0)
		prefixMatch.AddShould(ownerNamePrefix)

		// Fuzzy search for typos
		fuzzyQuery := bleve.NewFuzzyQuery(queryStringLower)
		fuzzyQuery.SetField("stand_number")
		fuzzyQuery.SetBoost(4.0)
		fuzzyQuery.SetFuzziness(1)
		prefixMatch.AddShould(fuzzyQuery)

		// Combine search strategies
		booleanQuery.AddShould(exactMatch)
		booleanQuery.AddShould(prefixMatch)
	}

	// Build final query with filters
	finalQuery := bleve.NewBooleanQuery()
	if queryString != "" {
		finalQuery.AddMust(booleanQuery)
	}

	// Add filters
	if status != "" {
		statusQuery := bleve.NewTermQuery(strings.ToLower(status))
		statusQuery.SetField("status")
		finalQuery.AddMust(statusQuery)
	}

	if standType != "" {
		typeQuery := bleve.NewTermQuery(strings.ToLower(standType))
		typeQuery.SetField("stand_type")
		finalQuery.AddMust(typeQuery)
	}

	if standCurrency != "" {
		currencyQuery := bleve.NewTermQuery(strings.ToLower(standCurrency))
		currencyQuery.SetField("stand_currency")
		finalQuery.AddMust(currencyQuery)
	}

	if active != nil {
		activeQuery := bleve.NewBoolFieldQuery(*active)
		activeQuery.SetField("is_active")
		finalQuery.AddMust(activeQuery)
	}

	return r.indexer.SearchIndex("stands", finalQuery, 20)
}

// UpdateStand updates a stand document in Bleve
func (r *BleveRepository) UpdateStand(stand models.Stand) error {
	standID := stand.ID.String()

	// Delete existing document
	if err := r.indexer.DeleteDocument("stands", standID); err != nil {
		config.Logger.Error("Failed to delete stand document for update",
			zap.Error(err),
			zap.String("stand_id", standID))
		return err
	}

	// Re-index updated stand
	return r.IndexSingleStand(stand)
}

// DeleteStand removes a stand from the index
func (r *BleveRepository) DeleteStand(standID string) error {
	err := r.indexer.DeleteDocument("stands", standID)
	if err != nil {
		config.Logger.Error("Failed to delete stand from Bleve",
			zap.Error(err),
			zap.String("stand_id", standID))
		return err
	}
	config.Logger.Info("Successfully deleted stand from Bleve",
		zap.String("stand_id", standID))
	return nil
}

func (r *BleveRepository) GetStandDocument(id string) (interface{}, error) {
	return r.indexer.GetDocument("stands", id)
}

func derefUUID(u *uuid.UUID) string {
	if u != nil {
		return u.String()
	}
	return ""
}

func getStandTypeName(standType *models.StandType) string {
	if standType != nil && standType.Name != "" {
		return standType.Name
	}
	return ""
}

func (r *BleveRepository) DebugStandIndex() error {
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 100
	searchRequest.Fields = []string{"*"}

	results, err := r.indexer.SearchIndex("stands", query, 100)
	if err != nil {
		return err
	}

	config.Logger.Info("Debug: All indexed Stands",
		zap.Int("total", int(results.Total)))

	for _, hit := range results.Hits {
		config.Logger.Info("Debug: Indexed Stands",
			zap.String("id", hit.ID),
			zap.Any("fields", hit.Fields))
	}

	return nil
}