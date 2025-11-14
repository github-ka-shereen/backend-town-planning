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

// DevelopmentPermitData holds all data for the development permit template
type DevelopmentPermitData struct {
	LogoBase64           string
	PrintDate            string
	PlanNumber           string
	StandNumber          string
	ApplicantName        string
	ApplicantAddress     string
	ApplicantCity        string
	PermitNumber         string
	PermitGenerationDate string
	ApplicationDate      string
	SubmissionDate       string
	DevelopmentType      string
	DevelopmentCategory  string
	PlanArea             string
	EstimatedCost        string
	DevelopmentLevy      string
	VATAmount            string
	TotalCost            string
	Currency             string
	ArchitectName        string
	ArchitectEmail       string
	ArchitectPhone       string
	Conditions           []string
	GeneratedByName      string
	GeneratedByTitle     string
	GeneratedBySignature string
	StampSpace           bool
	LegalNotice          string
}

// GenerateDevelopmentPermit generates a PDF development permit for the application
func GenerateDevelopmentPermit(application models.Application, finalApproval models.FinalApproval, filename string, user *models.User) (string, error) {
	// Prepare data for template
	permitData, err := prepareDevelopmentPermitData(application, finalApproval, user)
	if err != nil {
		return "", fmt.Errorf("failed to prepare development permit data: %v", err)
	}

	// Generate HTML content
	htmlContent, err := generateHTMLDevelopmentPermit(permitData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML development permit: %v", err)
	}

	// Generate PDF from HTML
	pdfPath, err := generateDevelopmentPermitPDFFromHTML(htmlContent, permitData, filename)
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %v", err)
	}

	return pdfPath, nil
}

// prepareDevelopmentPermitData prepares the data structure for the template
func prepareDevelopmentPermitData(application models.Application, finalApproval models.FinalApproval, user *models.User) (DevelopmentPermitData, error) {
	// Load logo
	logoBase64, err := loadMunicipalityLogo()
	if err != nil {
		config.Logger.Warn("Failed to load logo, using placeholder", zap.Error(err))
		logoBase64 = createMunicipalityPlaceholderLogo()
	}

	// Load user signature
	userSignature := ""
	if user.SignatureFilePath != nil {
		sigBase64, err := loadSignatureImage(*user.SignatureFilePath)
		if err != nil {
			config.Logger.Warn("Failed to load user signature", zap.String("path", *user.SignatureFilePath), zap.Error(err))
		} else {
			userSignature = sigBase64
		}
	}

	// Get development conditions based on category
	conditions := getDevelopmentConditions(application)

	// Format dates
	permitGenerationDate := formatDateFull(time.Now())

	submissionDate := formatDateFull(application.SubmissionDate)
	

	// Get currency from tariff
	currency := "USD" // default
	if application.Tariff != nil && application.Tariff.Currency != "" {
		currency = application.Tariff.Currency
	}

	// Format financial values with proper currency
	planArea := "N/A"
	if application.PlanArea != nil {
		planArea = fmt.Sprintf("%s m²", formatDecimal(*application.PlanArea))
	}

	estimatedCost := "N/A"
	if application.EstimatedCost != nil {
		estimatedCost = formatCurrency(*application.EstimatedCost, currency)
	}

	developmentLevy := "N/A"
	if application.DevelopmentLevy != nil {
		developmentLevy = formatCurrency(*application.DevelopmentLevy, currency)
	}

	vatAmount := "N/A"
	if application.VATAmount != nil {
		vatAmount = formatCurrency(*application.VATAmount, currency)
	}

	totalCost := "N/A"
	if application.TotalCost != nil {
		totalCost = formatCurrency(*application.TotalCost, currency)
	}

	// Get architect information
	architectName := "Not specified"
	architectEmail := "Not specified"
	architectPhone := "Not specified"
	if application.ArchitectFullName != nil {
		architectName = *application.ArchitectFullName
	}
	if application.ArchitectEmail != nil {
		architectEmail = *application.ArchitectEmail
	}
	if application.ArchitectPhoneNumber != nil {
		architectPhone = *application.ArchitectPhoneNumber
	}

	// Get development category
	developmentCategory := "Unknown"
	if application.Tariff != nil && application.Tariff.DevelopmentCategory.Name != "" {
		developmentCategory = application.Tariff.DevelopmentCategory.Name
	}

	// Get stand use/development type
	developmentType := "Unknown"
	if application.Stand != nil && application.Stand.StandType != nil {
		developmentType = application.Stand.StandType.Name
	}

	// Get applicant address and city
	applicantAddress := "Not specified"
	applicantCity := "Not specified"
	if application.Applicant.PostalAddress != nil {
		applicantAddress = *application.Applicant.PostalAddress
	}
	if application.Applicant.City != nil {
		applicantCity = *application.Applicant.City
	}

	return DevelopmentPermitData{
		LogoBase64:           logoBase64,
		PrintDate:            formatDateFull(time.Now()),
		PlanNumber:           application.PlanNumber,
		StandNumber:          getStandNumber(application),
		ApplicantName:        strings.ToUpper(application.Applicant.FullName),
		ApplicantAddress:     applicantAddress,
		ApplicantCity:        applicantCity,
		PermitNumber:         application.PermitNumber,
		PermitGenerationDate: permitGenerationDate,
		DevelopmentType:      developmentType,
		DevelopmentCategory:  developmentCategory,
		PlanArea:             planArea,
		SubmissionDate:       submissionDate,
		EstimatedCost:        estimatedCost,
		DevelopmentLevy:      developmentLevy,
		VATAmount:            vatAmount,
		TotalCost:            totalCost,
		ArchitectName:        architectName,
		ArchitectEmail:       architectEmail,
		ArchitectPhone:       architectPhone,
		Conditions:           conditions,
		GeneratedByName:      fmt.Sprintf("%s %s", user.FirstName, user.LastName),
		GeneratedByTitle:     getUserTitle(user),
		GeneratedBySignature: userSignature,
		StampSpace:           true,
		LegalNotice:          getLegalNotice(),
	}, nil
}

// formatCurrency formats a decimal value as currency with proper symbol and formatting
func formatCurrency(amount decimal.Decimal, currency string) string {
	// Format with 2 decimal places and thousands separators
	formatted := formatDecimal(amount)

	switch strings.ToUpper(currency) {
	case "USD":
		return fmt.Sprintf("$%s USD", formatted)
	case "ZWL":
		return fmt.Sprintf("ZWL$%s", formatted)
	case "ZAR":
		return fmt.Sprintf("R%s ZAR", formatted)
	case "BWP":
		return fmt.Sprintf("P%s BWP", formatted)
	default:
		return fmt.Sprintf("%s %s", formatted, currency)
	}
}

// formatDecimal formats a decimal with thousands separators and 2 decimal places
func formatDecimal(d decimal.Decimal) string {
	// Convert to float for formatting
	value := d.InexactFloat64()

	// Format with thousands separators and 2 decimal places
	return formatNumberWithCommas(value)
}

// formatNumberWithCommas formats a number with thousands separators
func formatNumberWithCommas(num float64) string {
	// Handle whole numbers and decimal numbers
	if num == float64(int64(num)) {
		return fmt.Sprintf("%d", int64(num))
	}

	// Format with 2 decimal places and commas for thousands
	str := fmt.Sprintf("%.2f", num)

	// Split integer and decimal parts
	parts := strings.Split(str, ".")
	if len(parts) != 2 {
		return str
	}

	integerPart := parts[0]
	decimalPart := parts[1]

	// Add commas to integer part
	var formattedInteger strings.Builder
	for i, char := range integerPart {
		if i > 0 && (len(integerPart)-i)%3 == 0 {
			formattedInteger.WriteString(",")
		}
		formattedInteger.WriteRune(char)
	}

	return fmt.Sprintf("%s.%s", formattedInteger.String(), decimalPart)
}

// getDevelopmentConditions returns appropriate conditions based on development category
func getDevelopmentConditions(application models.Application) []string {
	var conditions []string

	// Common conditions for all permits
	commonConditions := []string{
		"The development shall be implemented within 24 months.",
		"The design and siting of the building(s) shall be in accordance with the plans as approved by the Victoria Falls City Council and an occupation certificate issued prior to occupation of the buildings.",
	}

	conditions = append(conditions, commonConditions...)

	// Category-specific conditions
	if application.Tariff != nil {
		category := application.Tariff.DevelopmentCategory.Name

		switch {
		case strings.Contains(strings.ToLower(category), "commercial") || strings.Contains(strings.ToLower(category), "industrial"):
			conditions = append(conditions,
				"Materials used should be standard brick under tile or chromadek sheets. (Roof colors; green, grey/thatch).",
				"Boundary walls must not exceed 1.8m unless otherwise approved by Victoria Falls City Council.",
			)

		case strings.Contains(strings.ToLower(category), "residential"):
			conditions = append(conditions,
				"Standard brick under tile, Chroma deck sheets, Thatch, Hipped roof.",
				"Only earthly colors are permitted.",
				"Walls, no bright colors are permitted e.g. pink, orange.",
				"Roofs strictly colors that blend well with the environment e.g. black, grey and green.",
				"VFCC would need to inspect roofing tiles before being mounted, to ensure enforcement.",
				"Boundary walls must not exceed 1.8m unless otherwise approved by Victoria Falls City Council.",
				"The stand should be used for single family dwelling only unless otherwise approved by Victoria Falls City Council.",
			)

		case strings.Contains(strings.ToLower(category), "holiday") || strings.Contains(strings.ToLower(category), "hotel"):
			conditions = append(conditions,
				"That the stand shall be used for Holiday homes and ancillary uses only.",
				"Any other uses would require the approval of the City Council, no other uses save for the above-mentioned shall be allowed.",
				"The maximum height of the building shall not exceed 11 meters and not more than 2 Storeys.",
				"Boundary walls should not exceed 2 metres.",
				"No buildings other than boundary walls or fences shall be erected within 9 meters of the road frontage, 5m of the rear boundary and 3 meters on the side boundaries.",
				"The building shall not cover more than 65% of the area of the stand and 30% of the site shall be set aside as open space.",
				"On-site parking shall be provided and 1 bay for every 35m² of the total floor area or alternatively 2.5 bays per unit.",
				"The Local Authority reserves the right to withdraw the permit upon any violations to the conditions stated.",
			)

		case strings.Contains(strings.ToLower(category), "buffer"):
			conditions = append(conditions,
				"The whole of the planned substructure shall be constructed to oversite concrete slab level.",
				"All substructure brickwork shall be of approved industrial standard bricks.",
				"Roof type: Hipped covered with Chromadek sheets or tiles in any of the following colors: - green, grey, black.",
				"Boundary walls must not exceed 1.8metres unless otherwise approved by Victoria Falls City Council.",
				"All properties facing the buffer shall erect at least a frontage security wall before construction of the house commences.",
				"The stand shall be used for single family dwelling unless otherwise approved by Victoria Falls City Council.",
			)

		case strings.Contains(strings.ToLower(category), "church"):
			conditions = append(conditions,
				"The whole of the planned substructure shall be constructed to oversite concrete slab level.",
				"All substructure brickwork shall be of approved industrial standard bricks.",
				"The premises MAY ONLY be used as a meeting facility upon attaining an occupation certificate from the City Council.",
			)
		}
	}

	return conditions
}

// getUserTitle returns the user's title based on their role/department
func getUserTitle(user *models.User) string {
	if user.Department != nil {
		return user.Department.Name
	}
	if user.Role != nil {
		return user.Role.Name
	}
	return "Authorized Officer"
}

// getLegalNotice returns the standard legal notice
func getLegalNotice() string {
	return `THE ATTENTION OF THE APPLICANT OF THE APPLICATION IS DRAWN TO THE FOLLOWING MATTERS:

(a) Any person aggrieved by any decision made or deemed to have been made by the Local Planning Authority in connection with this permit may in terms of Section 38 of the Act, appeal to the Administrative Court, P. O. Box CY 1364, Causeway. Such appeal should be lodged with the registrar within one month of the date of notification of the decision against which the appeal is made.

(b) Any action taken in pursuance of the granting of this permit within the period allowed for appeals shall be at risk of the person taking action concerned.

(c) Amendment of this permit can only be made as provided in Section 26(12) of the Act.

THIS PERMIT DOES NOT CONSTITUTE APPROVAL IN TERMS OF ANY MUNICIPALITY BYE-LAWS`
}

// generateHTMLDevelopmentPermit generates HTML from template
func generateHTMLDevelopmentPermit(data DevelopmentPermitData) (string, error) {
	// Create a custom template function map
	funcMap := template.FuncMap{
		"add1": func(i int) int {
			return i + 1
		},
	}

	// Parse template with custom functions
	tmpl, err := template.New("development-permit.html").Funcs(funcMap).ParseFiles("templates/development-permit.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse development permit template: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute development permit template: %v", err)
	}

	return buf.String(), nil
}

// generateDevelopmentPermitPDFFromHTML generates PDF from HTML and saves to file
func generateDevelopmentPermitPDFFromHTML(htmlContent string, permitData DevelopmentPermitData, filename string) (string, error) {
	var pdfBuffer bytes.Buffer
	err := GenerateDevelopmentPermitPDF(htmlContent, permitData, &pdfBuffer)
	if err != nil {
		return "", err
	}

	dirPath := "./public/development-permits"
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	fullPath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(fullPath, pdfBuffer.Bytes(), 0644); err != nil {
		return "", err
	}

	return "public/development-permits/" + filename, nil
}

// GenerateDevelopmentPermitPDF generates portrait PDF from HTML content
func GenerateDevelopmentPermitPDF(htmlContent string, permitData DevelopmentPermitData, w io.Writer) error {
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
		parts := strings.SplitN(permitData.LogoBase64, ",", 2)
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

	// Serve signature
	mux.HandleFunc("/signature", func(w http.ResponseWriter, r *http.Request) {
		if permitData.GeneratedBySignature == "" {
			http.NotFound(w, r)
			return
		}

		parts := strings.SplitN(permitData.GeneratedBySignature, ",", 2)
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
				WithPaperWidth(8.27).   // A4 Portrait width
				WithPaperHeight(11.69). // A4 Portrait height
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				WithLandscape(false). // Portrait mode
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
