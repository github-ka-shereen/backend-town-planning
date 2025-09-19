package repositories

import (
	"strings"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

func (r *BleveRepository) IndexSingleApplicant(applicant models.Applicant) error {
	bleveApplicantDoc := struct {
		ID            string                 `json:"id"`
		FirstName     string                 `json:"first_name,omitempty"`
		LastName      string                 `json:"last_name,omitempty"`
		Organisation  string                 `json:"organisation_name,omitempty"`
		Email         string                 `json:"email"`
		Phone         string                 `json:"phone_number"`
		FullName      string                 `json:"full_name"`
		Debtor        bool                   `json:"debtor"`
		Status        models.ApplicantStatus `json:"status"`
		ApplicantType models.ApplicantType   `json:"applicant_type"`
	}{
		ID:            applicant.ID.String(),
		FirstName:     derefString(applicant.FirstName),
		LastName:      derefString(applicant.LastName),
		Organisation:  derefString(applicant.OrganisationName),
		Email:         applicant.Email,
		Phone:         applicant.PhoneNumber,
		FullName:      applicant.FullName,
		Debtor:        applicant.Debtor,
		Status:        applicant.Status,
		ApplicantType: applicant.ApplicantType,
	}

	err := r.indexer.IndexDocument("applicants", applicant.ID.String(), bleveApplicantDoc)
	if err != nil {
		config.Logger.Error("Failed to index single applicant into Bleve", zap.Error(err), zap.String("applicant_id", applicant.ID.String()))
		return err
	}

	config.Logger.Info("Successfully indexed single applicant into Bleve", zap.String("applicant_id", applicant.ID.String()))
	return nil
}

func (r *BleveRepository) IndexExistingApplicants(applicants []models.Applicant) error {
	docsToBleveIndex := make(map[string]interface{})

	for _, applicant := range applicants {
		bleveApplicantDoc := struct {
			ID            string                 `json:"id"`
			FirstName     string                 `json:"first_name,omitempty"`
			LastName      string                 `json:"last_name,omitempty"`
			Organisation  string                 `json:"organisation_name,omitempty"`
			Email         string                 `json:"email"`
			Phone         string                 `json:"phone_number"`
			FullName      string                 `json:"full_name"`
			Debtor        bool                   `json:"debtor"`
			Status        models.ApplicantStatus `json:"status"`
			ApplicantType models.ApplicantType   `json:"applicant_type"`
		}{
			ID:            applicant.ID.String(),
			FirstName:     derefString(applicant.FirstName),
			LastName:      derefString(applicant.LastName),
			Organisation:  derefString(applicant.OrganisationName),
			Email:         applicant.Email,
			Phone:         applicant.PhoneNumber,
			FullName:      applicant.FullName,
			Debtor:        applicant.Debtor,
			Status:        applicant.Status,
			ApplicantType: applicant.ApplicantType,
		}

		docsToBleveIndex[applicant.ID.String()] = bleveApplicantDoc
	}

	if len(docsToBleveIndex) > 0 {
		config.Logger.Info("Attempting to bulk index applicants into Bleve", zap.Int("count", len(docsToBleveIndex)))
		err := r.indexer.BulkIndexDocuments("applicants", docsToBleveIndex)
		if err != nil {
			config.Logger.Error("Failed to bulk index applicants into Bleve", zap.Error(err))
			return err
		}
		config.Logger.Info("Successfully bulk indexed applicants into Bleve", zap.Int("count", len(docsToBleveIndex)))
	} else {
		config.Logger.Info("No applicants to index into Bleve.")
	}

	return nil
}

func (r *BleveRepository) SearchApplicants(
	queryString string,
	status string,
) (*bleve.SearchResult, error) {
	booleanQuery := bleve.NewBooleanQuery()

	// Standardize the query string (trim and lowercase)
	queryString = strings.TrimSpace(strings.ToLower(queryString))

	// 1. Exact Matches (Highest Priority)
	exactMatch := bleve.NewBooleanQuery()
	exactFields := []string{"full_name", "organisation_name", "email", "phone_number"}
	for _, field := range exactFields {
		termQuery := bleve.NewTermQuery(queryString)
		termQuery.SetField(field)
		termQuery.SetBoost(6.0)
		exactMatch.AddShould(termQuery)
	}

	// 2. Phrase Matches (High Priority)
	phraseMatch := bleve.NewBooleanQuery()
	phraseFields := []string{"full_name", "organisation_name", "first_name", "last_name"}
	for _, field := range phraseFields {
		phraseQuery := bleve.NewMatchPhraseQuery(queryString)
		phraseQuery.SetField(field)
		phraseQuery.SetBoost(5.0)
		phraseMatch.AddShould(phraseQuery)
	}

	// 3. Fuzzy Matching (Medium Priority)
	fuzzyMatch := bleve.NewBooleanQuery()
	fuzzyFields := []string{"full_name", "organisation_name", "first_name", "last_name", "email"}
	for _, field := range fuzzyFields {
		fuzzyQuery := bleve.NewFuzzyQuery(queryString)
		fuzzyQuery.SetField(field)
		fuzzyQuery.SetFuzziness(2) // Allow up to 2 character differences
		fuzzyQuery.SetBoost(3.0)
		fuzzyMatch.AddShould(fuzzyQuery)
	}

	// 4. Prefix Matching (Low Priority)
	prefixMatch := bleve.NewBooleanQuery()
	prefixFields := []string{"full_name", "organisation_name", "first_name", "last_name", "phone_number"}
	for _, field := range prefixFields {
		prefixQuery := bleve.NewPrefixQuery(queryString)
		prefixQuery.SetField(field)
		prefixQuery.SetBoost(2.0)
		prefixMatch.AddShould(prefixQuery)
	}

	// 5. Wildcard Matching (Lowest Priority)
	wildcardMatch := bleve.NewBooleanQuery()
	wildcardQuery := bleve.NewWildcardQuery("*" + queryString + "*")
	wildcardQuery.SetBoost(1.0)
	wildcardMatch.AddShould(wildcardQuery)

	// Combine all strategies
	booleanQuery.AddShould(exactMatch)
	booleanQuery.AddShould(phraseMatch)
	booleanQuery.AddShould(fuzzyMatch)
	booleanQuery.AddShould(prefixMatch)
	booleanQuery.AddShould(wildcardMatch)

	// Build final query with filters
	finalQuery := bleve.NewBooleanQuery()
	finalQuery.AddMust(booleanQuery) // Include original search strategies

	// Add status filter if provided
	if status != "" {
		statusQuery := bleve.NewTermQuery(strings.ToLower(status))
		statusQuery.SetField("status")
		finalQuery.AddMust(statusQuery)
	}

	// Configure search request with final query
	searchRequest := bleve.NewSearchRequest(finalQuery)
	searchRequest.Size = 20
	searchRequest.Fields = []string{"*"}
	searchRequest.Explain = true

	return r.indexer.SearchIndex("applicants", finalQuery, 20)
}

// UpdateApplicant updates an applicant document in the Bleve index
func (r *BleveRepository) UpdateApplicant(applicant models.Applicant) error {
	applicantID := applicant.ID.String()

	// 1. Delete the existing document
	err := r.indexer.DeleteDocument("applicants", applicantID)
	if err != nil {
		config.Logger.Error("Failed to delete applicant document for update in Bleve",
			zap.Error(err),
			zap.String("applicant_id", applicantID))
		return err
	}
	config.Logger.Info("Successfully deleted old applicant document for update in Bleve",
		zap.String("applicant_id", applicantID))

	// 2. Re-index the updated applicant
	err = r.IndexSingleApplicant(applicant) // Reuse your existing IndexSingleApplicant method
	if err != nil {
		config.Logger.Error("Failed to re-index updated applicant into Bleve",
			zap.Error(err),
			zap.String("applicant_id", applicantID))
		return err
	}

	config.Logger.Info("Successfully updated (re-indexed) applicant in Bleve",
		zap.String("applicant_id", applicantID))
	return nil
}

// DeleteApplicant removes an applicant document from the Bleve index
func (r *BleveRepository) DeleteApplicant(applicantID string) error {
	err := r.indexer.DeleteDocument("applicants", applicantID)
	if err != nil {
		config.Logger.Error("Failed to delete applicant from Bleve",
			zap.Error(err),
			zap.String("applicant_id", applicantID))
		return err
	}

	config.Logger.Info("Successfully deleted applicant from Bleve",
		zap.String("applicant_id", applicantID))
	return nil
}

func (r *BleveRepository) GetApplicantDocument(id string) (interface{}, error) {
	return r.indexer.GetDocument("applicants", id)
}

func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
