// Add this to a utils package or in the same file
package utils

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ExecuteParallel runs multiple database queries in parallel
func ExecuteParallel(queries ...func() error) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(queries))

	for _, query := range queries {
		wg.Add(1)
		go func(q func() error) {
			defer wg.Done()
			if err := q(); err != nil && err != gorm.ErrRecordNotFound {
				errChan <- err
			}
		}(query)
	}

	wg.Wait()
	close(errChan)

	// Return the first error encountered
	for err := range errChan {
		return err
	}

	return nil
}

// Helper function to safely dereference strings
func DerefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// Format time pointer to string
func FormatTimePointer(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format("2006-01-02T15:04:05Z07:00")
	return &formatted
}

// DecimalToString safely converts decimal to string
func DecimalToString(d *decimal.Decimal) *string {
	if d == nil {
		return nil
	}
	str := d.String()
	return &str
}
