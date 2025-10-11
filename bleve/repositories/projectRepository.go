// repositories/bleve_repository.go
package repositories

import (
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

func (r *BleveRepository) IndexSingleProject(project models.Project) error {
	bleveProjectDoc := struct {
		ID            string `json:"id"`
		ProjectName   string `json:"project_name"`
		ProjectNumber string `json:"project_number"`
		Address       string `json:"address"`
		City          string `json:"city"`
	}{
		ID:            project.ID.String(),
		ProjectName:   project.ProjectName,
		ProjectNumber: project.ProjectNumber,
		Address:       project.Address,
		City:          project.City,
	}

	err := r.indexer.IndexDocument("projects", project.ID.String(), bleveProjectDoc)
	if err != nil {
		config.Logger.Error("Failed to index single project into Bleve",
			zap.Error(err),
			zap.String("project_id", project.ID.String()))
		return err
	}

	config.Logger.Info("Successfully indexed single project into Bleve",
		zap.String("project_id", project.ID.String()))
	return nil
}

func (r *BleveRepository) IndexExistingProjects(projects []models.Project) error {
	docsToBleveIndex := make(map[string]interface{})

	for _, project := range projects {
		bleveProjectDoc := struct {
			ID            string `json:"id"`
			ProjectName   string `json:"project_name"`
			ProjectNumber string `json:"project_number"`
			Address       string `json:"address"`
			City          string `json:"city"`
		}{
			ID:            project.ID.String(),
			ProjectName:   project.ProjectName,
			ProjectNumber: project.ProjectNumber,
			Address:       project.Address,
			City:          project.City,
		}

		docsToBleveIndex[project.ID.String()] = bleveProjectDoc
	}

	if len(docsToBleveIndex) > 0 {
		config.Logger.Info("Attempting to bulk index projects into Bleve",
			zap.Int("count", len(docsToBleveIndex)))
		err := r.indexer.BulkIndexDocuments("projects", docsToBleveIndex)
		if err != nil {
			config.Logger.Error("Failed to bulk index projects into Bleve", zap.Error(err))
			return err
		}
		config.Logger.Info("Successfully bulk indexed projects into Bleve",
			zap.Int("count", len(docsToBleveIndex)))
	} else {
		config.Logger.Info("No projects to index into Bleve.")
	}

	return nil
}

func (r *BleveRepository) SearchProjects(
	queryString string,
	city string,
) (*bleve.SearchResult, error) {
	booleanQuery := bleve.NewBooleanQuery()
	queryString = strings.TrimSpace(queryString)
	queryStringLower := strings.ToLower(queryString)

	// Search strategies (prioritized)
	if queryString != "" {
		// 1. Exact matches with original case and lowercase
		exactMatch := bleve.NewBooleanQuery()

		// Project name exact matches
		projectNameExact := bleve.NewTermQuery(queryString)
		projectNameExact.SetField("project_name")
		projectNameExact.SetBoost(10.0)
		exactMatch.AddShould(projectNameExact)

		// Project number exact matches
		projectNumExact := bleve.NewTermQuery(queryString)
		projectNumExact.SetField("project_number")
		projectNumExact.SetBoost(9.5)
		exactMatch.AddShould(projectNumExact)

		// Lowercase versions
		projectNameExactLower := bleve.NewTermQuery(queryStringLower)
		projectNameExactLower.SetField("project_name")
		projectNameExactLower.SetBoost(9.0)
		exactMatch.AddShould(projectNameExactLower)

		projectNumExactLower := bleve.NewTermQuery(queryStringLower)
		projectNumExactLower.SetField("project_number")
		projectNumExactLower.SetBoost(8.5)
		exactMatch.AddShould(projectNumExactLower)

		// 2. Match query for analyzed fields (handles tokenization)
		matchQuery := bleve.NewMatchQuery(queryString)
		matchQuery.SetField("project_name")
		matchQuery.SetBoost(7.0)
		exactMatch.AddShould(matchQuery)

		matchQueryNum := bleve.NewMatchQuery(queryString)
		matchQueryNum.SetField("project_number")
		matchQueryNum.SetBoost(6.5)
		exactMatch.AddShould(matchQueryNum)

		// 3. Prefix matches
		prefixMatch := bleve.NewBooleanQuery()

		projectNamePrefix := bleve.NewPrefixQuery(queryStringLower)
		projectNamePrefix.SetField("project_name")
		projectNamePrefix.SetBoost(6.0)
		prefixMatch.AddShould(projectNamePrefix)

		projectNumPrefix := bleve.NewPrefixQuery(queryStringLower)
		projectNumPrefix.SetField("project_number")
		projectNumPrefix.SetBoost(5.5)
		prefixMatch.AddShould(projectNumPrefix)

		// 4. Fuzzy search for typos
		fuzzyQuery := bleve.NewFuzzyQuery(queryStringLower)
		fuzzyQuery.SetField("project_name")
		fuzzyQuery.SetBoost(4.0)
		fuzzyQuery.SetFuzziness(1) // Allow 1 character difference
		prefixMatch.AddShould(fuzzyQuery)

		// 5. Wildcard queries for partial matches
		wildcardQueries := bleve.NewBooleanQuery()

		// Try various wildcard patterns
		patterns := []string{
			"*" + queryStringLower + "*", // Contains
			queryStringLower + "*",       // Starts with
			"*" + queryStringLower,       // Ends with
		}

		for i, pattern := range patterns {
			wildcardQuery := bleve.NewWildcardQuery(pattern)
			wildcardQuery.SetField("project_name")
			wildcardQuery.SetBoost(3.0 - float64(i)*0.5) // Decreasing boost
			wildcardQueries.AddShould(wildcardQuery)
		}

		// Combine all search strategies
		booleanQuery.AddShould(exactMatch)
		booleanQuery.AddShould(prefixMatch)
		booleanQuery.AddShould(wildcardQueries)
	}

	// Build final query with filters
	finalQuery := bleve.NewBooleanQuery()
	if queryString != "" {
		finalQuery.AddMust(booleanQuery)
	}

	// Add city filter
	if city != "" {
		cityQuery := bleve.NewTermQuery(strings.ToLower(city))
		cityQuery.SetField("city")
		finalQuery.AddMust(cityQuery)
	}

	return r.indexer.SearchIndex("projects", finalQuery, 20)
}

func (r *BleveRepository) UpdateProject(project models.Project) error {
	projectID := project.ID.String()

	// Delete existing document
	if err := r.indexer.DeleteDocument("projects", projectID); err != nil {
		config.Logger.Error("Failed to delete project document for update",
			zap.Error(err),
			zap.String("project_id", projectID))
		return err
	}

	// Re-index updated project
	return r.IndexSingleProject(project)
}

func (r *BleveRepository) DeleteProject(projectID string) error {
	err := r.indexer.DeleteDocument("projects", projectID)
	if err != nil {
		config.Logger.Error("Failed to delete project from Bleve",
			zap.Error(err),
			zap.String("project_id", projectID))
		return err
	}
	config.Logger.Info("Successfully deleted project from Bleve",
		zap.String("project_id", projectID))
	return nil
}

func (r *BleveRepository) GetProjectDocument(id string) (interface{}, error) {
	return r.indexer.GetDocument("projects", id)
}
