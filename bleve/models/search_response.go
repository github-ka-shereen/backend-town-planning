// models/search_response.go
package models

type SearchHit struct {
    ID     string                 `json:"id"`
    Fields map[string]interface{} `json:"fields,omitempty"`
}

type SearchResponse struct {
    Hits []SearchHit `json:"hits"`
}
