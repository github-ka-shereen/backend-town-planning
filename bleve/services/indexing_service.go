package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

type IndexingServiceInterface interface {
	IndexDocument(indexName, id string, document interface{}) error
	BulkIndexDocuments(indexName string, documents map[string]interface{}) error
	DeleteDocument(indexName, id string) error
	SearchIndex(indexName string, q query.Query, size int) (*bleve.SearchResult, error)
	GetIndex(indexName string) (bleve.Index, error)
	GetDocument(indexName, id string) (interface{}, error)
	UpdateDocument(indexName, id string, document interface{}) error
	DeleteIndex(indexName string) error
	IndexExists(indexName string) (bool, error)
	DeleteAllIndices() error
}

type IndexingService struct {
	indexes  map[string]bleve.Index
	logger   *zap.Logger
	basePath string
}

func NewIndexingService(logger *zap.Logger, basePath string) *IndexingService {
	return &IndexingService{
		indexes:  make(map[string]bleve.Index),
		logger:   logger,
		basePath: basePath, // Initialize basePath
	}
}

func (s *IndexingService) GetIndex(indexName string) (bleve.Index, error) {
	return s.getOrCreateIndex(indexName)
}

func (s *IndexingService) getOrCreateIndex(indexName string) (bleve.Index, error) {
	if idx, ok := s.indexes[indexName]; ok {
		return idx, nil
	}

	// Construct the full path using basePath
	fullPath := fmt.Sprintf("%s/%s.bleve", s.basePath, indexName)

	// Define index mapping to store all fields for retrieval
	mapping := bleve.NewIndexMapping()
	// You may want to customize mapping to set Store:true on all fields

	idx, err := bleve.Open(fullPath) // Use fullPath
	if err != nil {
		// If index does not exist, create a new one
		idx, err = bleve.New(fullPath, mapping) // Use fullPath
		if err != nil {
			return nil, fmt.Errorf("failed to create index %s: %w", fullPath, err)
		}
	}

	s.indexes[indexName] = idx
	return idx, nil
}

// SearchIndex performs a search and requests stored fields to be included
func (s *IndexingService) SearchIndex(indexName string, q query.Query, size int) (*bleve.SearchResult, error) {
	idx, err := s.getOrCreateIndex(indexName)
	if err != nil {
		s.logger.Error("Could not get or create index", zap.Error(err))
		return nil, err
	}

	searchRequest := bleve.NewSearchRequestOptions(q, size, 0, false)
	searchRequest.Fields = []string{"*"} // Fetch all stored fields

	searchResult, err := idx.Search(searchRequest)
	if err != nil {
		s.logger.Error("Search failed", zap.Error(err))
		return nil, err
	}

	return searchResult, nil
}

func (s *IndexingService) IndexDocument(indexName, id string, document interface{}) error {
	idx, err := s.getOrCreateIndex(indexName)
	if err != nil {
		s.logger.Error("Could not get or create index", zap.Error(err))
		return err
	}

	if err := idx.Index(id, document); err != nil {
		s.logger.Error("Failed to index document", zap.String("id", id), zap.Error(err))
		return err
	}

	s.logger.Info("Successfully indexed document", zap.String("id", id))
	return nil
}

func (s *IndexingService) BulkIndexDocuments(indexName string, documents map[string]interface{}) error {
	idx, err := s.getOrCreateIndex(indexName)
	if err != nil {
		s.logger.Error("Could not get or create index", zap.Error(err))
		return err
	}

	batch := idx.NewBatch()
	for id, doc := range documents {
		if err := batch.Index(id, doc); err != nil {
			s.logger.Error("Failed to add doc to batch", zap.String("id", id), zap.Error(err))
			return err
		}
	}

	if err := idx.Batch(batch); err != nil {
		s.logger.Error("Failed to execute batch", zap.Error(err))
		return err
	}

	s.logger.Info("Successfully bulk indexed documents", zap.Int("count", len(documents)))
	return nil
}

func (s *IndexingService) DeleteDocument(indexName, id string) error {
	idx, err := s.getOrCreateIndex(indexName)
	if err != nil {
		s.logger.Error("Could not get or create index", zap.Error(err))
		return err
	}

	if err := idx.Delete(id); err != nil {
		s.logger.Error("Failed to delete document", zap.String("id", id), zap.Error(err))
		return err
	}

	s.logger.Info("Successfully deleted document", zap.String("id", id))
	return nil
}

func (s *IndexingService) UpdateDocument(indexName, id string, document interface{}) error {
	s.logger.Info("Attempting to update document", zap.String("id", id))

	// 1. Delete the old document
	if err := s.DeleteDocument(indexName, id); err != nil {
		s.logger.Error("Failed to delete document for update", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("failed to delete existing document for update: %w", err)
	}

	// 2. Index the new (updated) document
	if err := s.IndexDocument(indexName, id, document); err != nil {
		s.logger.Error("Failed to re-index document after deletion", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("failed to re-index updated document: %w", err)
	}

	s.logger.Info("Successfully updated document", zap.String("id", id))
	return nil
}

// GetDocument tries to get stored fields from a document by searching by ID
func (s *IndexingService) GetDocument(indexName, id string) (interface{}, error) {
	idx, err := s.getOrCreateIndex(indexName)
	if err != nil {
		return nil, err
	}

	// Search by ID to retrieve stored fields
	query := bleve.NewDocIDQuery([]string{id})
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 1
	searchRequest.Fields = []string{"*"} // fetch stored fields

	searchResult, err := idx.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(searchResult.Hits) == 0 {
		return nil, fmt.Errorf("document not found")
	}

	return searchResult.Hits[0].Fields, nil
}

func (s *IndexingService) DeleteIndex(indexName string) error {
	idx, exists := s.indexes[indexName]
	if !exists {
		return fmt.Errorf("index %s not found in memory", indexName)
	}

	// Close the index first
	if err := idx.Close(); err != nil {
		s.logger.Error("Failed to close index before deletion",
			zap.String("index_name", indexName),
			zap.Error(err))
		return fmt.Errorf("failed to close index: %w", err)
	}

	// Remove from memory
	delete(s.indexes, indexName)

	// Delete the physical index files
	fullPath := fmt.Sprintf("%s/%s.bleve", s.basePath, indexName)
	if err := os.RemoveAll(fullPath); err != nil {
		s.logger.Error("Failed to delete index files",
			zap.String("path", fullPath),
			zap.Error(err))
		return fmt.Errorf("failed to delete index files: %w", err)
	}

	s.logger.Info("Successfully deleted index",
		zap.String("index_name", indexName))
	return nil
}

func (s *IndexingService) IndexExists(indexName string) (bool, error) {
	fullPath := fmt.Sprintf("%s/%s.bleve", s.basePath, indexName)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *IndexingService) DeleteAllIndices() error {
	// Get list of known indices from your service
	knownIndices := make([]string, 0, len(s.indexes))
	for indexName := range s.indexes {
		knownIndices = append(knownIndices, indexName)
	}

	var errorsOccurred []error
	var successCount int

	for _, indexName := range knownIndices {
		if err := s.DeleteIndex(indexName); err != nil {
			errorsOccurred = append(errorsOccurred, err)
			continue
		}
		successCount++
	}

	// Check for any filesystem indices we might have missed
	files, err := filepath.Glob(filepath.Join(s.basePath, "*.bleve"))
	if err != nil {
		s.logger.Error("Failed to scan for index files",
			zap.String("path", s.basePath),
			zap.Error(err))
		return fmt.Errorf("failed to scan index directory: %w", err)
	}

	for _, file := range files {
		indexName := strings.TrimSuffix(filepath.Base(file), ".bleve")
		if _, exists := s.indexes[indexName]; !exists {
			if err := os.RemoveAll(file); err != nil {
				errorsOccurred = append(errorsOccurred, err)
				continue
			}
			successCount++
			s.logger.Info("Deleted orphaned index files",
				zap.String("index_name", indexName))
		}
	}

	if len(errorsOccurred) > 0 {
		s.logger.Error("Some indices failed to delete",
			zap.Int("success_count", successCount),
			zap.Int("error_count", len(errorsOccurred)))
		return fmt.Errorf("%d errors occurred while deleting indices (%d succeeded)",
			len(errorsOccurred), successCount)
	}

	s.logger.Info("All indices deleted successfully",
		zap.Int("count", successCount))
	return nil
}
