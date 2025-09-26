package repositories

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

func (r *BleveRepository) IndexSingleVATRate(vatRate models.VATRate) error {
	bleveVATRateDoc := struct {
		ID        string  `json:"id"`
		Rate      float64 `json:"rate"`
		ValidFrom string  `json:"valid_from"`
		ValidTo   string  `json:"valid_to,omitempty"`
		IsActive  bool    `json:"is_active"`
		Used      bool    `json:"used"`
	}{
		ID:        vatRate.ID.String(),
		Rate:      vatRate.Rate.InexactFloat64(),
		ValidFrom: vatRate.ValidFrom.Format(time.RFC3339),
		IsActive:  vatRate.IsActive,
		Used:      vatRate.Used,
	}

	// If ValidTo is not nil, format it
	if vatRate.ValidTo != nil {
		bleveVATRateDoc.ValidTo = vatRate.ValidTo.Format(time.RFC3339)
	}

	err := r.indexer.IndexDocument("vat_rates", vatRate.ID.String(), bleveVATRateDoc)
	if err != nil {
		config.Logger.Error("Failed to index single VAT rate into Bleve",
			zap.Error(err),
			zap.String("vat_rate_id", vatRate.ID.String()))
		return err
	}

	config.Logger.Info("Successfully indexed single VAT rate into Bleve",
		zap.String("vat_rate_id", vatRate.ID.String()))
	return nil
}

func (r *BleveRepository) IndexExistingVATRates(vatRates []models.VATRate) error {
	docsToBleveIndex := make(map[string]interface{})

	for _, vatRate := range vatRates {
		doc := struct {
			ID        string  `json:"id"`
			Rate      float64 `json:"rate"`
			ValidFrom string  `json:"valid_from"`
			ValidTo   string  `json:"valid_to,omitempty"`
			IsActive  bool    `json:"is_active"`
			Used      bool    `json:"used"`
		}{
			ID:        vatRate.ID.String(),
			Rate:      vatRate.Rate.InexactFloat64(),
			ValidFrom: vatRate.ValidFrom.Format(time.RFC3339),
			IsActive:  vatRate.IsActive,
			Used:      vatRate.Used,
		}

		if vatRate.ValidTo != nil {
			doc.ValidTo = vatRate.ValidTo.Format(time.RFC3339)
		}

		docsToBleveIndex[vatRate.ID.String()] = doc
	}

	if len(docsToBleveIndex) > 0 {
		config.Logger.Info("Attempting to bulk index VAT rates into Bleve",
			zap.Int("count", len(docsToBleveIndex)))
		err := r.indexer.BulkIndexDocuments("vat_rates", docsToBleveIndex)
		if err != nil {
			config.Logger.Error("Failed to bulk index VAT rates into Bleve", zap.Error(err))
			return err
		}
		config.Logger.Info("Successfully bulk indexed VAT rates into Bleve",
			zap.Int("count", len(docsToBleveIndex)))
	} else {
		config.Logger.Info("No VAT rates to index into Bleve.")
	}

	return nil
}

func (r *BleveRepository) SearchVATRates(
	is_active *bool,
	used *bool,
	sortBy []string,
) (*bleve.SearchResult, error) {
	finalQuery := bleve.NewBooleanQuery()

	// Active filter
	if is_active != nil {
		activeQuery := bleve.NewBoolFieldQuery(*is_active)
		activeQuery.SetField("is_active")
		finalQuery.AddMust(activeQuery)
	}

	// Used filter
	if used != nil {
		usedQuery := bleve.NewBoolFieldQuery(*used)
		usedQuery.SetField("used")
		finalQuery.AddMust(usedQuery)
	}

	// Configure search request
	searchRequest := bleve.NewSearchRequest(finalQuery)
	searchRequest.Size = 20
	searchRequest.Fields = []string{"*"}
	searchRequest.Explain = true

	// Apply sorting - default to valid_from descending if no sort specified
	if len(sortBy) > 0 {
		searchRequest.SortBy(sortBy)
	} else {
		searchRequest.SortBy([]string{"-valid_from"}) // Default sort by valid_from descending
	}

	return r.indexer.SearchIndex("vat_rates", finalQuery, 20)
}

func (r *BleveRepository) UpdateVATRate(vatRate models.VATRate) error {
	vatRateID := vatRate.ID.String()

	// Delete existing document
	if err := r.indexer.DeleteDocument("vat_rates", vatRateID); err != nil {
		config.Logger.Error("Failed to delete VAT rate document for update",
			zap.Error(err),
			zap.String("vat_rate_id", vatRateID))
		return err
	}

	// Re-index updated VAT rate
	return r.IndexSingleVATRate(vatRate)
}

func (r *BleveRepository) DeleteVATRate(vatRateID string) error {
	err := r.indexer.DeleteDocument("vat_rates", vatRateID)
	if err != nil {
		config.Logger.Error("Failed to delete VAT rate from Bleve",
			zap.Error(err),
			zap.String("vat_rate_id", vatRateID))
		return err
	}
	config.Logger.Info("Successfully deleted VAT rate from Bleve",
		zap.String("vat_rate_id", vatRateID))
	return nil
}

func (r *BleveRepository) GetVATRateDocument(id string) (interface{}, error) {
	return r.indexer.GetDocument("vat_rates", id)
}

func (r *BleveRepository) DebugVATRateIndex() error {
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 100
	searchRequest.Fields = []string{"*"}

	results, err := r.indexer.SearchIndex("vat_rates", query, 100)
	if err != nil {
		return err
	}

	config.Logger.Info("Debug: All indexed VAT rates",
		zap.Int("total", int(results.Total)))

	for _, hit := range results.Hits {
		config.Logger.Info("Debug: Indexed VAT rate",
			zap.String("id", hit.ID),
			zap.Any("fields", hit.Fields))
	}

	return nil
}
