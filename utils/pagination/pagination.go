package pagination

import (
	"fmt"
	"math"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type PaginationParams struct {
    Page     int               `json:"page"`
    PageSize int              `json:"page_size"`
    Filters  map[string]string `json:"filters"`
}

type PaginationMeta struct {
    CurrentPage int     `json:"current_page"`
    PageSize    int     `json:"page_size"`
    TotalPages  int     `json:"total_pages"`
    TotalItems  int64   `json:"total_items"`
    NextPage    *string `json:"next_page"`
    PrevPage    *string `json:"prev_page"`
}

type PaginatedResponse struct {
    Items      interface{}   `json:"items"`
    Pagination PaginationMeta `json:"pagination"`
}

func ParsePaginationParams(c *fiber.Ctx) PaginationParams {
    page := c.QueryInt("page", 1)
    pageSize := c.QueryInt("page_size", 10)
    
    filters := make(map[string]string)
    c.Context().QueryArgs().VisitAll(func(key, value []byte) {
        k := string(key)
        if k != "page" && k != "page_size" {
            filters[k] = string(value)
        }
    })

    return PaginationParams{
        Page:     page,
        PageSize: pageSize,
        Filters:  filters,
    }
}

func ValidatePaginationParams(params PaginationParams) error {
    if params.Page < 1 {
        return fmt.Errorf("page must be greater than 0")
    }
    if params.PageSize < 1 || params.PageSize > 100 {
        return fmt.Errorf("page size must be between 1 and 100")
    }
    return nil
}

func buildPaginationURL(c *fiber.Ctx, page int, params PaginationParams) string {
    baseURL := fmt.Sprintf("%s://%s%s?", c.Protocol(), c.Hostname(), c.Path())
    queryParams := make([]string, 0)
    
    if params.PageSize != 10 {
        queryParams = append(queryParams, fmt.Sprintf("page_size=%d", params.PageSize))
    }
    
    for key, value := range params.Filters {
        if value != "" {
            queryParams = append(queryParams, fmt.Sprintf("%s=%s", key, value))
        }
    }
    
    queryParams = append(queryParams, fmt.Sprintf("page=%d", page))
    return baseURL + strings.Join(queryParams, "&")
}

func NewPaginatedResponse(c *fiber.Ctx, items interface{}, totalItems int64, params PaginationParams) PaginatedResponse {
    totalPages := int(math.Ceil(float64(totalItems) / float64(params.PageSize)))
    
    var nextPageURL, prevPageURL *string
    
    if params.Page < totalPages {
        next := buildPaginationURL(c, params.Page+1, params)
        nextPageURL = &next
    }
    
    if params.Page > 1 {
        prev := buildPaginationURL(c, params.Page-1, params)
        prevPageURL = &prev
    }
    
    return PaginatedResponse{
        Items: items,
        Pagination: PaginationMeta{
            CurrentPage: params.Page,
            PageSize:    params.PageSize,
            TotalPages:  totalPages,
            TotalItems:  totalItems,
            NextPage:    nextPageURL,
            PrevPage:    prevPageURL,
        },
    }
}

// CheckPageSize is a utility function to check if PageSize is greater than a given threshold
func CheckPageSizeForDownload(params fiber.Map, threshold int) bool {
	pageSize, ok := params["page_size"].(int)
	if !ok {
		// Default to 0 if PageSize is not provided or invalid
		pageSize = 0
	}
	return pageSize > threshold
}

// CheckTotalResultsForDownload checks if the total number of results exceeds 5
func CheckTotalResultsForDownload(currentNumberOfResults int) bool {
	return currentNumberOfResults > 5
}
