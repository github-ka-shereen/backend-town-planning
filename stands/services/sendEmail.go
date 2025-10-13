package services

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
)

// Load environment variables from .env file
func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
}

// Initialize the SMTP mailer once and store it in a global variable
var mailer *gomail.Dialer

// InitializeMailer sets up the mailer using environment variables
func InitializeMailer() {
	LoadEnv()

	mailHost := os.Getenv("SMTP_HOST")
	mailPort := os.Getenv("SMTP_PORT")
	mailUser := os.Getenv("SMTP_USER")
	mailPassword := os.Getenv("SMTP_PASSWORD")

	port, err := strconv.Atoi(mailPort)
	if err != nil {
		log.Fatalf("Invalid SMTP_PORT value: %v, defaulting to port 25", err)
		port = 25
	}

	mailer = gomail.NewDialer(mailHost, port, mailUser, mailPassword)
}

// GetMailer returns the initialized mailer
func GetMailer() *gomail.Dialer {
	return mailer
}

// SendEmail sends an email with an optional OTP and attachment, and returns an error if it fails.
func SendEmail(email string, message string, title string, otp string, attachmentPath string) error {
	if mailer == nil {
		return fmt.Errorf("mailer is not initialized")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_FROM"))
	m.SetHeader("To", email)
	m.SetHeader("Subject", title)

	// If OTP is provided, include it in the message
	if otp != "" {
		m.SetBody("text/plain", fmt.Sprintf("%s\nYour OTP is: %s", message, otp))
		m.SetBody("text/html", fmt.Sprintf(`
		<html>
		<head>
			<meta charset="utf-8">
			<title>Your OTP Code</title>
		</head>
		<body style="margin: 0; padding: 0; background-color: #f4f4f4;">
			<table width="100%%" border="0" cellspacing="0" cellpadding="0">
				<tr>
					<td>
						<table width="600" border="0" cellspacing="0" cellpadding="0" style="margin: 0 auto; background-color: #ffffff;">
							<tr>
								<td style="padding: 20px;">
									<h1 style="color: #333; font-family: Arial, sans-serif;">Welcome to AcrePoint!</h1>
									<p style="color: #555; font-family: Arial, sans-serif;">Your Two Factor Authentication Code is <strong>%s</strong></p>
								</td>
							</tr>
							<tr>
								<td style="padding: 20px; text-align: center; font-size: 12px; color: #aaa;">
									<p>&copy; 2024 Acre Point. All rights reserved.</p>
								</td>
							</tr>
						</table>
					</td>
				</tr>
			</table>
		</body>
		</html>`, otp))
	} else {
		// If no OTP, send only the message with download link if provided
		m.SetBody("text/plain", message)
		m.SetBody("text/html", fmt.Sprintf(`
		<html>
		<head>
			<meta charset="utf-8">
			<title>%s</title>
		</head>
		<body>
			<p>%s</p>
			<p><a href="%s">Download the file here</a></p>
		</body>
		</html>`, title, message, attachmentPath))
	}

	// Attach file if path is provided
	if attachmentPath != "" {
		if _, err := os.Stat(attachmentPath); err == nil {
			m.Attach(attachmentPath)
		} else {
			fmt.Println("Attachment file not found:", attachmentPath)
		}
	}

	// Send the email and return any error
	if err := mailer.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Println("Email sent successfully!")
	return nil // return nil if email sent successfully
}
