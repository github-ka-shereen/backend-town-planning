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

// TPD1FormData holds all data needed for the TPD-1 form template
type TPD1FormData struct {
	LogoBase64             string
	FormDate               string
	ReceiptNumber          string
	DateReceived           string
	PermitNumber           string
	RegisteredNumber       string
	FeeSubmitted           string
	ApplicantSurname       string
	ApplicantOtherNames    string
	ApplicantPostalAddress string
	ApplicantTelephone     string
	ApplicantTitle         string
	OwnerSurname           string
	OwnerOtherNames        string
	OwnerPostalAddress     string
	PropertyDescription    string
	StreetAddress          string
	PropertyArea           string
	HasTitleRestrictions   bool
	ProposedDevelopment    string
	LocalPlanningAuthority string
	PaymentMethod          string
	ApplicationDate        string
	ApplicantSignature     string
	AgentName              string
	AgentAddress           string
	AgentTelephone         string
	OwnerSignature         string
}

// GenerateTPD1Form generates a PDF TPD-1 form for development permit application
func GenerateTPD1Form(application models.Application, filename string) (string, error) {
	// Prepare data for template
	formData, err := prepareTPD1FormData(application)
	if err != nil {
		return "", fmt.Errorf("failed to prepare TPD-1 form data: %v", err)
	}

	// Generate HTML content
	htmlContent, err := generateHTMLTPD1Form(formData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML TPD-1 form: %v", err)
	}

	// Generate PDF from HTML
	pdfPath, err := generateTPD1PDFFromHTML(htmlContent, formData.LogoBase64, filename)
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %v", err)
	}

	return pdfPath, nil
}

// prepareTPD1FormData prepares the data structure for the TPD-1 form template
func prepareTPD1FormData(application models.Application) (TPD1FormData, error) {
	// Load logo
	logoBase64, err := loadMunicipalityLogo()
	if err != nil {
		config.Logger.Warn("Failed to load logo, using placeholder", zap.Error(err))
		logoBase64 = createMunicipalityPlaceholderLogo()
	}

	// Determine applicant title
	title := ""

	// Only proceed if gender and marital status exist
	if application.Applicant.Gender != nil {
		gender := *application.Applicant.Gender
		var marital string
		if application.Applicant.MaritalStatus != nil {
			marital = *application.Applicant.MaritalStatus
		}

		switch gender {
		case "Male":
			title = "Mr"
		case "Female":
			if marital == "Married" {
				title = "Mrs"
			} else if marital == "Single" {
				title = "Ms"
			} else {
				// leave blank if marital status is unknown
				title = ""
			}
		default:
			title = "" // unknown gender
		}
	}

	// Format full name parts
	surname, otherNames := splitFullName(application.Applicant.FullName)

	// Get owner details (if different from applicant)
	ownerSurname := ""
	ownerOtherNames := ""
	ownerPostalAddress := ""
	if application.Applicant.FullName != "" {
		ownerSurname, ownerOtherNames = splitFullName(application.Applicant.FullName)
		ownerPostalAddress = *application.Applicant.PostalAddress
	}

	// Format property area
	propertyArea := ""
	if application.PlanArea != nil {
		propertyArea = fmt.Sprintf("%.2f m²", application.PlanArea.InexactFloat64())
	}

	return TPD1FormData{
		LogoBase64:             logoBase64,
		FormDate:               time.Now().Format("02-January-2006"),
		ReceiptNumber:          getReceiptNumber(application),
		DateReceived:           application.SubmissionDate.Format("02-January-2006"),
		PermitNumber:           application.PermitNumber,
		RegisteredNumber:       application.PlanNumber,
		FeeSubmitted:           formatFeeAmount(application),
		ApplicantSurname:       surname,
		ApplicantOtherNames:    otherNames,
		ApplicantPostalAddress: *application.Applicant.PostalAddress,
		ApplicantTelephone:     application.Applicant.PhoneNumber,
		ApplicantTitle:         title,
		OwnerSurname:           ownerSurname,
		OwnerOtherNames:        ownerOtherNames,
		OwnerPostalAddress:     ownerPostalAddress,
		PropertyDescription:    getPropertyDescription(application),
		// StreetAddress:          application.Stand.StreetAddress,
		PropertyArea:           propertyArea,
		HasTitleRestrictions:   false, // Default to false, can be enhanced
		LocalPlanningAuthority: "Municipality of Redcliff",
		PaymentMethod:          "",
		ApplicationDate:        application.SubmissionDate.Format("02-January-2006"),
		ApplicantSignature:     "", // To be signed physically
		AgentName:              "",
		AgentAddress:           "",
		AgentTelephone:         "",
		OwnerSignature:         "", // To be signed physically
	}, nil
}

// Helper functions for TPD-1 form data extraction

func splitFullName(fullName string) (surname, otherNames string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[len(parts)-1], strings.Join(parts[:len(parts)-1], " ")
}

func getReceiptNumber(application models.Application) string {

	// Option 1: Return the latest paid payment's receipt number
	if application.Payment.ReceiptNumber != "" {
		return application.Payment.ReceiptNumber
	}

	return ""
}

func formatFeeAmount(application models.Application) string {
	if application.TotalCost != nil {
		return fmt.Sprintf("%s %.2f", application.Tariff.Currency, application.TotalCost.InexactFloat64())
	}
	return "N/A"
}

func getPropertyDescription(application models.Application) string {
	if application.Stand.StandNumber != "" {
		return fmt.Sprintf("Stand %s", application.Stand.StandNumber)
	}
	return "N/A"
}

// generateHTMLTPD1Form generates HTML content from the TPD-1 form template
func generateHTMLTPD1Form(data TPD1FormData) (string, error) {
	// Parse template file
	tmpl, err := template.ParseFiles("templates/tpd1-form.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse TPD-1 form template: %v", err)
	}

	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute TPD-1 form template: %v", err)
	}

	return buf.String(), nil
}

// loadMunicipalityLogo loads the municipality logo
func loadMunicipalityLogo() (string, error) {
	logoPaths := []string{
		"./templates/logo/redcliff_logo.jpg",
		"./templates/logo/redcliff_logo.png",
	}

	for _, logoPath := range logoPaths {
		if _, err := os.Stat(logoPath); err == nil {
			config.Logger.Info("Loading municipality logo", zap.String("path", logoPath))
			return encodeImageToBase64(logoPath)
		}
	}

	return "", fmt.Errorf("no municipality logo file found")
}

// createMunicipalityPlaceholderLogo creates a placeholder logo
func createMunicipalityPlaceholderLogo() string {
	svg := `<svg width="75" height="75" xmlns="http://www.w3.org/2000/svg">
                <rect width="75" height="75" fill="#0066cc" rx="5"/>
                <text x="37.5" y="40" font-family="Arial" font-size="14" fill="white" text-anchor="middle">LOGO</text>
            </svg>`

	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// generateTPD1PDFFromHTML generates PDF from HTML content and saves to file
func generateTPD1PDFFromHTML(htmlContent, logoBase64, filename string) (string, error) {
	// Generate PDF bytes
	var pdfBuffer bytes.Buffer
	err := GenerateTPD1PDF(htmlContent, logoBase64, &pdfBuffer)
	if err != nil {
		return "", err
	}

	// Save PDF to file
	dirPath := "./public/tpd1-forms"
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	fullPath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(fullPath, pdfBuffer.Bytes(), 0644); err != nil {
		return "", err
	}

	return "public/tpd1-forms/" + filename, nil
}

// GenerateTPD1PDF generates PDF from HTML content with A4 format
func GenerateTPD1PDF(htmlContent, logoBase64 string, w io.Writer) error {
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
		parts := strings.SplitN(logoBase64, ",", 2)
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
				WithMarginTop(0.6).
				WithMarginBottom(0.6).
				WithMarginLeft(0.6).
				WithMarginRight(0.6).
				WithPreferCSSPageSize(true).
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
