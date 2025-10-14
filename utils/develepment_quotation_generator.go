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
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// QuotationData holds all data needed for the quotation template
type QuotationData struct {
	LogoBase64          string
	DeveloperName       string
	StandNumber         string
	PlanNumber          string
	PermitNumber        string
	DateReceived        string
	PermitType          string
	DevelopmentType     string
	PrintDate           string
	PlanArea            string
	PricePerSquareMeter string
	EstimatedCost       string
	PermitFee           string
	InspectionFee       string
	DevelopmentFee      string
	VATAmount           string
	TotalFees           string
}

// GenerateDevelopmentPermitQuotation generates a PDF quotation for development permit fees
func GenerateDevelopmentPermitQuotation(application models.Application, filename string) (string, error) {
	// Prepare data for template using the application data that's already calculated
	quotationData, err := prepareQuotationData(application)
	if err != nil {
		return "", fmt.Errorf("failed to prepare quotation data: %v", err)
	}

	// Generate HTML content
	htmlContent, err := generateHTMLQuotation(quotationData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML quotation: %v", err)
	}

	// Generate PDF from HTML
	pdfPath, err := generateA4PDFFromHTMLFile(htmlContent, quotationData.LogoBase64, filename)
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %v", err)
	}

	return pdfPath, nil
}

// prepareQuotationData prepares the data structure for the quotation template using existing application data
func prepareQuotationData(application models.Application) (QuotationData, error) {
	// Load logo
	logoBase64, err := loadVictoriaFallsLogo()
	if err != nil {
		config.Logger.Warn("Failed to load logo, using placeholder", zap.Error(err))
		logoBase64 = createVictoriaFallsPlaceholderLogo()
	}

	// Use the data that's already calculated and stored in the application
	return QuotationData{
		LogoBase64:          logoBase64,
		DeveloperName:       getDeveloperName(application),
		StandNumber:         getStandNumber(application),
		PlanNumber:          application.PlanNumber,
		PermitNumber:        application.PermitNumber,
		DateReceived:        application.SubmissionDate.Format("02/01/2006"),
		PermitType:          getPermitType(application),
		DevelopmentType:     getDevelopmentType(application),
		PrintDate:           time.Now().Format("02/01/2006"),
		PlanArea:            formatCurrency(*application.PlanArea),
		PricePerSquareMeter: formatCurrency(application.Tariff.PricePerSquareMeter),
		EstimatedCost:       formatCurrency(*application.EstimatedCost),
		PermitFee:           formatCurrency(application.Tariff.PermitFee),
		InspectionFee:       formatCurrency(application.Tariff.InspectionFee),
		DevelopmentFee:      formatCurrency(*application.DevelopmentLevy),
		VATAmount:           formatCurrency(*application.VATAmount),
		TotalFees:           formatCurrency(*application.TotalCost),
	}, nil
}

// Helper functions to extract data from application
func getDeveloperName(application models.Application) string {
	if application.Applicant.FullName != "" {
		return application.Applicant.FullName
	}
	if application.Applicant.FullName != "" {
		return application.Applicant.FullName
	}
	return "Unknown Developer"
}

func getStandNumber(application models.Application) string {
	if application.Stand.StandNumber != "" {
		return application.Stand.StandNumber
	}
	return "N/A"
}

func getPermitType(application models.Application) string {
	// Use the tariff's development category to determine permit type
	if application.Tariff.DevelopmentCategory.Name != "" {
		return application.Tariff.DevelopmentCategory.Name
	}
	return "Commercial & Industrial"
}

func getDevelopmentType(application models.Application) string {
	// Use the tariff's development category for development type
	if application.Tariff.DevelopmentCategory.Name != "" {
		return strings.ToUpper(application.Tariff.DevelopmentCategory.Name)
	}
	return "COMMERCIAL"
}

// generateHTMLQuotation generates HTML content from the quotation template
func generateHTMLQuotation(data QuotationData) (string, error) {
	// Parse template file
	tmpl, err := template.ParseFiles("templates/development-application-quotation.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse quotation template: %v", err)
	}

	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute quotation template: %v", err)
	}

	return buf.String(), nil
}

// loadVictoriaFallsLogo loads the Victoria Falls municipality logo
func loadVictoriaFallsLogo() (string, error) {
	logoPaths := []string{
		"./templates/logo/redcliff_logo.jpg",
		"./templates/logo/redcliff_logo.png",
	}

	for _, logoPath := range logoPaths {
		if _, err := os.Stat(logoPath); err == nil {
			config.Logger.Info("Loading Redcliff logo", zap.String("path", logoPath))
			return encodeImageToBase64(logoPath)
		}
	}

	return "", fmt.Errorf("no Redcliff logo file found")
}

// createVictoriaFallsPlaceholderLogo creates a placeholder logo for Victoria Falls
func createVictoriaFallsPlaceholderLogo() string {
	svg := `<svg width="60" height="60" xmlns="http://www.w3.org/2000/svg">
                <rect width="60" height="60" fill="#0066cc" rx="5"/>
                <text x="30" y="35" font-family="Arial" font-size="12" fill="white" text-anchor="middle">LOGO</text>
            </svg>`

	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// formatCurrency formats decimal to currency string
func formatCurrency(amount decimal.Decimal) string {
	amountStr := amount.StringFixed(2)
	parts := strings.Split(amountStr, ".")
	intPart := parts[0]

	var formattedInt string
	for i, c := range reverseString(intPart) {
		if i > 0 && i%3 == 0 {
			formattedInt = "," + formattedInt
		}
		formattedInt = string(c) + formattedInt
	}

	if len(parts) > 1 {
		return formattedInt + "." + parts[1]
	}
	return formattedInt
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// encodeImageToBase64 encodes an image file to base64
func encodeImageToBase64(imagePath string) (string, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %v", err)
	}

	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
		mimeType = "image/png"
	} else if strings.HasSuffix(strings.ToLower(imagePath), ".gif") {
		mimeType = "image/gif"
	}

	base64String := base64.StdEncoding.EncodeToString(imageData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64String)

	return dataURI, nil
}

// generateA4PDFFromHTMLFile generates A4 PDF from HTML content and saves to file
func generateA4PDFFromHTMLFile(htmlContent, logoBase64, filename string) (string, error) {
	// Generate PDF bytes
	var pdfBuffer bytes.Buffer
	err := GenerateA4PDFFromHTML(htmlContent, logoBase64, &pdfBuffer)
	if err != nil {
		return "", err
	}

	// Save PDF to file
	dirPath := "./public/quotations"
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	fullPath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(fullPath, pdfBuffer.Bytes(), 0644); err != nil {
		return "", err
	}

	return "public/quotations/" + filename, nil
}

// GenerateA4PDFFromHTML generates PDF from HTML content with A4 format
func GenerateA4PDFFromHTML(htmlContent, logoBase64 string, w io.Writer) error {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create a multiplexer to handle both HTML and image requests
	mux := http.NewServeMux()

	// Serve the HTML content
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(htmlContent))
	})

	// Serve the logo as a separate endpoint
	mux.HandleFunc("/logo", func(w http.ResponseWriter, r *http.Request) {
		// Extract MIME type and base64 data from the data URI
		parts := strings.SplitN(logoBase64, ",", 2)
		if len(parts) != 2 {
			http.Error(w, "Invalid logo data", http.StatusInternalServerError)
			return
		}

		// Extract MIME type
		mimeParts := strings.SplitN(parts[0], ";", 2)
		mimeType := strings.TrimPrefix(mimeParts[0], "data:")

		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			http.Error(w, "Failed to decode logo", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", mimeType)
		_, _ = w.Write(data)
	})

	// Start the server on a random port
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
				WithPaperWidth(8.27).
				WithPaperHeight(11.7).
				WithMarginTop(0.4).
				WithMarginBottom(0.6).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				WithPreferCSSPageSize(true).
				WithDisplayHeaderFooter(true).
				WithHeaderTemplate(`<div style="font-size: 12px; width: 100%; text-align: center;"></div>`).
				WithFooterTemplate(`<div style="font-size: 12px; width: 100%; text-align: center; margin: 0 auto;">Page <span class="pageNumber"></span> of <span class="totalPages"></span></div>`).
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
