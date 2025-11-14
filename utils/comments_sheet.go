package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// FinalComment represents the final comment from each approval group member
type FinalComment struct {
	SectionName      string
	Date             *time.Time
	Status           string
	ReviewerName     string
	Comment          string
	CommentCreatedAt time.Time
	DepartmentName   string
	SignaturePath    string
}

// CommentsSheetData holds all data for the comments sheet template
type CommentsSheetData struct {
	LogoBase64          string
	PrintDate           string
	PlanNumber          string
	StandNumber         string
	DeveloperName       string
	DevelopmentPermitNo string
	StandUse            string
	ValueSubmitted      string
	DevelopmentFee      string
	DateSubmitted       string
	Location            string
	Comments            []CommentRow
	ApprovalStatus      string
	DateApproved        string
}

// CommentRow represents a single row in the comments sheet
type CommentRow struct {
	SectionName     string
	Date            string
	Status          string
	StatusFormatted string
	ReviewerName    string
	Comment         string
	HasSignature    bool
	SignatureBase64 string // Add this field for signature images
}

// GenerateCommentsSheet generates a PDF comments sheet for the application
func GenerateCommentsSheet(application models.Application, finalComments []FinalComment, filename string, user *models.User) (string, error) {
	// Prepare data for template
	sheetData, err := prepareCommentsSheetData(application, finalComments, user)
	if err != nil {
		return "", fmt.Errorf("failed to prepare comments sheet data: %v", err)
	}

	// Generate HTML content
	htmlContent, err := generateHTMLCommentsSheet(sheetData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML comments sheet: %v", err)
	}

	// Generate PDF from HTML
	pdfPath, err := generateCommentsSheetPDFFromHTML(htmlContent, sheetData, filename)
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %v", err)
	}

	return pdfPath, nil
}

// formatStatus formats the status for display
func formatStatus(status string) string {
	switch status {
	case "APPROVED":
		return "APPROVED"
	case "REJECTED":
		return "REJECTED"
	case "PENDING":
		return "PENDING"
	default:
		return status
	}
}

// calculateOverallStatus determines the overall approval status
func calculateOverallStatus(comments []FinalComment) (string, string) {
	hasRejection := false
	allApproved := true
	hasPending := false

	for _, comment := range comments {
		if comment.Status == "REJECTED" {
			hasRejection = true
			allApproved = false
		} else if comment.Status == "PENDING" {
			hasPending = true
			allApproved = false
		} else if comment.Status != "APPROVED" {
			allApproved = false
		}
	}

	if hasRejection {
		return "REJECTED", ""
	}
	if hasPending {
		return "PENDING", ""
	}
	if allApproved {
		return "APPROVED", formatDateFull(time.Now())
	}

	return "PENDING", ""
}

// formatDateFull formats a date in full format like "Wednesday, November 12th, 2025 at 6:09:31 PM"
func formatDateFull(date time.Time) string {
	// Get the day with ordinal suffix
	day := date.Day()
	suffix := getOrdinalSuffix(day)

	return date.Format(fmt.Sprintf("Monday, January 2%s, 2006 at 3:04:05 PM", suffix))
}

// getOrdinalSuffix returns the ordinal suffix for a day (st, nd, rd, th)
func getOrdinalSuffix(day int) string {
	switch day {
	case 1, 21, 31:
		return "st"
	case 2, 22:
		return "nd"
	case 3, 23:
		return "rd"
	default:
		return "th"
	}
}

// prepareCommentsSheetData prepares the data structure for the template
func prepareCommentsSheetData(application models.Application, finalComments []FinalComment, user *models.User) (CommentsSheetData, error) {
	// Load logo
	logoBase64, err := loadMunicipalityLogo()
	if err != nil {
		config.Logger.Warn("Failed to load logo, using placeholder", zap.Error(err))
		logoBase64 = createMunicipalityPlaceholderLogo()
	}

	// Format comments with signatures
	commentRows := make([]CommentRow, 0, len(finalComments))
	for _, fc := range finalComments {
		dateStr := ""
		if fc.Date != nil {
			dateStr = formatDateFull(*fc.Date)
		}

		// Format status display
		statusFormatted := formatStatus(fc.Status)

		// Load signature if exists
		signatureBase64 := ""
		if fc.SignaturePath != "" {
			sigBase64, err := loadSignatureImage(fc.SignaturePath)
			if err != nil {
				config.Logger.Warn("Failed to load signature", zap.String("path", fc.SignaturePath), zap.Error(err))
			} else {
				signatureBase64 = sigBase64
			}
		}

		commentRows = append(commentRows, CommentRow{
			SectionName:     strings.ToUpper(fc.DepartmentName), // Use department name in section
			Date:            dateStr,
			Status:          fc.Status,
			StatusFormatted: statusFormatted,
			ReviewerName:    fc.ReviewerName,
			Comment:         fc.Comment,
			HasSignature:    fc.Status == "APPROVED" || fc.Status == "REJECTED",
			SignatureBase64: signatureBase64,
		})
	}

	// Calculate overall approval status
	approvalStatus, dateApproved := calculateOverallStatus(finalComments)

	// Format financial values
	valueSubmitted := "N/A"
	developmentFee := "N/A"
	if application.TotalCost != nil {
		cost := application.TotalCost.InexactFloat64()
		valueSubmitted = fmt.Sprintf("$ %.2f", cost)
		developmentFee = fmt.Sprintf("$%.2f", cost*0.03) // Assuming 3% fee
	}

	// Format submission date in full
	submissionDateFormatted := formatDateFull(application.SubmissionDate)

	// Format print date in full
	printDateFormatted := formatDateFull(time.Now())

	return CommentsSheetData{
		LogoBase64:          logoBase64,
		PrintDate:           printDateFormatted,
		PlanNumber:          application.PlanNumber,
		StandNumber:         getStandNumber(application),
		DeveloperName:       strings.ToUpper(application.Applicant.FullName),
		DevelopmentPermitNo: application.PermitNumber,
		StandUse:            getStandUse(application),
		ValueSubmitted:      valueSubmitted,
		DevelopmentFee:      developmentFee,
		DateSubmitted:       submissionDateFormatted,
		Location:            getLocation(application),
		Comments:            commentRows,
		ApprovalStatus:      approvalStatus,
		DateApproved:        dateApproved,
	}, nil
}

// loadSignatureImage loads and converts signature image to base64
func loadSignatureImage(filePath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("signature file does not exist: %s", filePath)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read signature file: %v", err)
	}

	// Determine MIME type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	var mimeType string
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".svg":
		mimeType = "image/svg+xml"
	default:
		return "", fmt.Errorf("unsupported signature file format: %s", ext)
	}

	// Convert to base64
	base64Str := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str), nil
}

// getStandUse extracts stand use information safely
func getStandUse(application models.Application) string {
	if application.Stand != nil && application.Stand.StandType != nil && application.Stand.StandType.Name != "" {
		return strings.ToUpper(application.Stand.StandType.Name)
	}
	return "Unknown"
}

// getLocation extracts location information safely
func getLocation(application models.Application) string {
	if application.Tariff != nil && application.Tariff.DevelopmentCategory.Name != "" {
		return strings.ToUpper(application.Tariff.DevelopmentCategory.Name)
	}
	return "Unknown"
}

// generateHTMLCommentsSheet generates HTML from template
func generateHTMLCommentsSheet(data CommentsSheetData) (string, error) {
	tmpl, err := template.ParseFiles("templates/comments-sheet.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse comments sheet template: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute comments sheet template: %v", err)
	}

	return buf.String(), nil
}

// generateCommentsSheetPDFFromHTML generates PDF from HTML and saves to file
func generateCommentsSheetPDFFromHTML(htmlContent string, sheetData CommentsSheetData, filename string) (string, error) {
	var pdfBuffer bytes.Buffer
	err := GenerateCommentsSheetPDF(htmlContent, sheetData, &pdfBuffer)
	if err != nil {
		return "", err
	}

	dirPath := "./public/comments-sheets"
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	fullPath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(fullPath, pdfBuffer.Bytes(), 0644); err != nil {
		return "", err
	}

	return "public/comments-sheets/" + filename, nil
}

// GenerateCommentsSheetPDF generates landscape PDF from HTML content
func GenerateCommentsSheetPDF(htmlContent string, sheetData CommentsSheetData, w io.Writer) error {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	mux := http.NewServeMux()

	// Serve HTML content
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(htmlContent))
	})

	// Serve logo
	mux.HandleFunc("/logo", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(sheetData.LogoBase64, ",", 2)
		if len(parts) != 2 {
			http.Error(w, "Invalid logo data", http.StatusInternalServerError)
			return
		}

		mimeParts := strings.SplitN(parts[0], ";", 2)
		mimeType := strings.TrimPrefix(mimeParts[0], "data:")

		data, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			http.Error(w, "Failed to decode logo", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", mimeType)
		_, _ = w.Write(data)
	})

	// Serve signature images
	mux.HandleFunc("/signature/", func(w http.ResponseWriter, r *http.Request) {
		// Extract signature index from URL
		path := r.URL.Path
		indexStr := strings.TrimPrefix(path, "/signature/")

		// Find the comment with this signature
		for i, comment := range sheetData.Comments {
			if fmt.Sprintf("%d", i) == indexStr && comment.SignatureBase64 != "" {
				parts := strings.SplitN(comment.SignatureBase64, ",", 2)
				if len(parts) != 2 {
					http.Error(w, "Invalid signature data", http.StatusInternalServerError)
					return
				}

				mimeParts := strings.SplitN(parts[0], ";", 2)
				mimeType := strings.TrimPrefix(mimeParts[0], "data:")

				data, err := base64.StdEncoding.DecodeString(parts[1])
				if err != nil {
					http.Error(w, "Failed to decode signature", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", mimeType)
				_, _ = w.Write(data)
				return
			}
		}

		http.NotFound(w, r)
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	defer listener.Close()

	go server.Serve(listener)
	defer server.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://localhost:%d", port)

	var buf []byte
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.WaitReady("img"),
		chromedp.Sleep(2*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(11.7).  // A4 Landscape width
				WithPaperHeight(8.27). // A4 Landscape height
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				WithLandscape(true). // Enable landscape mode
				WithPreferCSSPageSize(false).
				WithDisplayHeaderFooter(false).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return err
	}

	_, err = w.Write(buf)
	return err
}
