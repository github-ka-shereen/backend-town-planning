package repositories

import (
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

func (r *BleveRepository) SearchUsers(queryString string) (*bleve.SearchResult, error) {
	// Create a "boolean query" to combine different search strategies.
	// A boolean query allows us to define "AND", "OR", and "NOT" conditions.
	// Here, we'll use "OR" (AddShould) to find results that match ANY of our strategies.
	booleanQuery := bleve.NewBooleanQuery()

	// Define the specific fields within a user document that we want to search through.
	// These should be the fields you indexed (e.g., in IndexExistingUsers).
	fieldsToSearch := []string{"first_name", "last_name", "email", "phone"}

	// --- Define the different types of search queries we'll use ---

	// 1. Exact Match Query:
	// This finds documents where the search term matches exactly with a word in the field.
	// Example: "John" will match "John", but not "Johnny" or "Jhon".
	matchQuery := bleve.NewMatchQuery(queryString)
	matchQuery.SetBoost(3.0) // Give exact matches the highest importance (boost) in results.

	// 2. Prefix Match Query:
	// This finds documents where words in the field start with the search term.
	// Example: "jo" will match "John", "Jonathan", "Joseph".
	prefixQuery := bleve.NewPrefixQuery(queryString)
	prefixQuery.SetBoost(2.0) // Give prefix matches a good, but slightly lower importance than exact.

	// 3. Fuzzy Match Query (for typo tolerance):
	// This finds documents where words are "close enough" to the search term, allowing for small typos.
	// SetFuzziness(1) allows for 1 character difference (e.g., 'a' instead of 'e', or one extra letter).
	// Example: "jhon" will match "John", "recieve" might match "receive".
	fuzzyQuery := bleve.NewFuzzyQuery(queryString)
	fuzzyQuery.SetFuzziness(1) // Allows 1 character edit distance (typo)
	fuzzyQuery.SetBoost(1.0)   // Give fuzzy matches the lowest importance, as they are less precise.

	// --- Apply each query type to all the specified fields ---
	for _, field := range fieldsToSearch {
		// Create an exact match query specifically for the current field.
		fieldMatchQuery := bleve.NewMatchQuery(queryString)
		fieldMatchQuery.SetField(field)              // Target this specific field
		fieldMatchQuery.SetBoost(matchQuery.Boost()) // Apply the exact match boost
		booleanQuery.AddShould(fieldMatchQuery)      // Add it as an "OR" condition

		// Create a prefix match query specifically for the current field.
		fieldPrefixQuery := bleve.NewPrefixQuery(queryString)
		fieldPrefixQuery.SetField(field)               // Target this specific field
		fieldPrefixQuery.SetBoost(prefixQuery.Boost()) // Apply the prefix match boost
		booleanQuery.AddShould(fieldPrefixQuery)       // Add it as an "OR" condition

		// Create a fuzzy match query specifically for the current field.
		fieldFuzzyQuery := bleve.NewFuzzyQuery(queryString)
		fieldFuzzyQuery.SetField(field) // Target this specific field
		// Set the fuzziness level from our defined fuzzyQuery (accessing the field directly).
		fieldFuzzyQuery.SetFuzziness(fuzzyQuery.Fuzziness)
		fieldFuzzyQuery.SetBoost(fuzzyQuery.Boost()) // Apply the fuzzy match boost
		booleanQuery.AddShould(fieldFuzzyQuery)      // Add it as an "OR" condition
	}

	// Set MinShould(1): This means that at least one of the "OR" conditions (any match, prefix, or fuzzy in any field)
	// must be true for a document to be considered a result.
	booleanQuery.SetMinShould(1)

	// Execute the constructed search query against the "users" index.
	// The last parameter (20) limits the number of search results returned.
	return r.indexer.SearchIndex("users", booleanQuery, 20)
}

func (r *BleveRepository) GetUserDocument(id string) (interface{}, error) {
	return r.indexer.GetDocument("users", id)
}

func (r *BleveRepository) IndexSingleUser(user models.User) error {
	bleveUserDoc := struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
	}{
		ID:        user.ID.String(),
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Phone:     user.Phone,
	}

	err := r.indexer.IndexDocument("users", user.ID.String(), bleveUserDoc)
	if err != nil {
		config.Logger.Error("Failed to index single user into Bleve", zap.Error(err), zap.String("user_id", user.ID.String()))
		return err
	}

	config.Logger.Info("Successfully indexed single user into Bleve", zap.String("user_id", user.ID.String()))
	return nil
}

// UpdateUser deletes the existing user document and re-indexes the updated user.
func (r *BleveRepository) UpdateUser(user models.User) error {
	userID := user.ID.String()

	// 1. Delete the existing document
	err := r.indexer.DeleteDocument("users", userID)
	if err != nil {
		config.Logger.Error("Failed to delete user document for update in Bleve", zap.Error(err), zap.String("user_id", userID))
		return err
	}
	config.Logger.Info("Successfully deleted old user document for update in Bleve", zap.String("user_id", userID))

	// 2. Re-index the updated user
	err = r.IndexSingleUser(user) // Reuse the existing IndexSingleUser method
	if err != nil {
		config.Logger.Error("Failed to re-index updated user into Bleve", zap.Error(err), zap.String("user_id", userID))
		return err
	}

	config.Logger.Info("Successfully updated (re-indexed) user in Bleve", zap.String("user_id", userID))
	return nil
}

// DeleteUser removes a user document from the Bleve "users" index.
func (r *BleveRepository) DeleteUser(userID string) error {
	err := r.indexer.DeleteDocument("users", userID)
	if err != nil {
		config.Logger.Error("Failed to delete user from Bleve", zap.Error(err), zap.String("user_id", userID))
		return err
	}

	config.Logger.Info("Successfully deleted user from Bleve", zap.String("user_id", userID))
	return nil
}

// IndexExistingUsers indexes a slice of user models into the Bleve "users" index
func (r *BleveRepository) IndexExistingUsers(users []models.User) error {
	docsToBleveIndex := make(map[string]interface{})

	for _, user := range users {
		// Define what fields from your User model you want to index and store
		// Ensure these fields align with what you want to search by.
		bleveUserDoc := struct {
			ID        string `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
			Phone     string `json:"phone"` // Ensure this matches your search field
			// Add any other user fields you want to be searchable or retrievable
			// e.g., CreatedAt: user.CreatedAt, LastLogin: user.LastLogin
		}{
			ID:        user.ID.String(),
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Phone:     user.Phone, // Assuming this is the field name in your users_repositories.User
		}
		docsToBleveIndex[user.ID.String()] = bleveUserDoc
	}

	if len(docsToBleveIndex) > 0 {
		config.Logger.Info("Attempting to bulk index users into Bleve", zap.Int("count", len(docsToBleveIndex)))
		err := r.indexer.BulkIndexDocuments("users", docsToBleveIndex) // "users" is the index name
		if err != nil {
			config.Logger.Error("Failed to bulk index existing users into Bleve", zap.Error(err))
			return err
		}
		config.Logger.Info("Successfully bulk indexed existing users into Bleve", zap.Int("count", len(docsToBleveIndex)))
	} else {
		config.Logger.Info("No existing users to index into Bleve.")
	}
	return nil
}
