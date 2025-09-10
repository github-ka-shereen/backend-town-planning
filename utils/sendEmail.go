package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"town-planning-backend/config" // Import your config package to access config.Logger

	// Added for InitializeDateLocation, if not already there in original
	"go.uber.org/zap" // Import zap for structured logging fields
	"gopkg.in/gomail.v2"
)

// Initialize the SMTP mailer once and store it in a global variable
var mailer *gomail.Dialer

// InitializeMailer sets up the mailer using environment variables
func InitializeMailer() {
	// No need to call LoadEnv() here, as it should be handled once at application startup (e.g., in main.go)
	// If you still need to load it here for some reason, ensure it's not redundant.

	mailHost := os.Getenv("SMTP_HOST")
	mailPort := os.Getenv("SMTP_PORT")
	mailUser := os.Getenv("SMTP_USER")
	mailPassword := os.Getenv("SMTP_PASSWORD")
	// smtpFrom := os.Getenv("SMTP_FROM") // Also retrieve SMTP_FROM here

	// Basic validation for critical mailer settings
	// if mailHost == "" || mailPort == "" || mailUser == "" || mailPassword == "" || smtpFrom == "" {
	// 	config.Logger.Fatal("Missing one or more SMTP environment variables (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD, SMTP_FROM)",
	// 		zap.Strings("missing_vars", getMissingSMTPEnvVars(mailHost, mailPort, mailUser, mailPassword, smtpFrom)),
	// 	)
	// }

	port, err := strconv.Atoi(mailPort)
	if err != nil {
		config.Logger.Error("Invalid SMTP_PORT value, defaulting to port 25",
			zap.String("provided_port", mailPort),
			zap.Error(err),
		)
		port = 25 // Fallback to a default port if conversion fails
	}

	mailer = gomail.NewDialer(mailHost, port, mailUser, mailPassword)
	config.Logger.Info("Mailer initialized successfully")
}

// // Helper function to identify missing SMTP environment variables for logging
// func getMissingSMTPEnvVars(host, port, user, pass, from string) []string {
// 	missing := []string{}
// 	if host == "" {
// 		missing = append(missing, "SMTP_HOST")
// 	}
// 	if port == "" {
// 		missing = append(missing, "SMTP_PORT")
// 	}
// 	if user == "" {
// 		missing = append(missing, "SMTP_USER")
// 	}
// 	if pass == "" {
// 		missing = append(missing, "SMTP_PASSWORD")
// 	}
// 	if from == "" {
// 		missing = append(missing, "SMTP_FROM")
// 	}
// 	return missing
// }

// GetMailer returns the initialized mailer
func GetMailer() *gomail.Dialer {
	return mailer
}

// SendEmail sends an email with an optional OTP and attachment, and returns an error if it fails.
func SendEmail(email string, message string, title string, otp string, attachmentPath string) error {
	if mailer == nil {
		err := fmt.Errorf("mailer is not initialized")
		config.Logger.Error("Email send failed: mailer is not initialized",
			zap.String("to_email", email),
			zap.String("subject", title),
			zap.Bool("has_otp", otp != ""),
			zap.Bool("has_attachment", attachmentPath != ""),
			zap.Error(err),
		)
		return err
	}

	m := gomail.NewMessage()
	// Using the retrieved SMTP_FROM from InitializeMailer
	m.SetHeader("From", os.Getenv("SMTP_FROM")) // Ensure SMTP_FROM is set and valid
	m.SetHeader("To", email)
	m.SetHeader("Subject", title)
	if otp != "" {
		lines := strings.Split(message, "\n")
		var link string
		for _, line := range lines {
			if strings.HasPrefix(line, "http") {
				link = line
				break
			}
		}

		if link != "" {
			m.SetBody("text/plain", fmt.Sprintf("%s\nYour OTP is: %s", message, otp))
			m.SetBody("text/html", fmt.Sprintf(`
				<html>
					<head>
						<meta charset="utf-8">
						<title>Your OTP Code and Password Reset Link</title>
					</head>
					<body>
						<p>Your OTP (Verification code): <strong>%s</strong></p>
						<p>You have requested a password reset. Please click on the link below to reset your password</p>
						<p>This link is valid for 5 minutes. If you did not request this, please ignore this email</p>
						<p><a href="%s" target="_blank">Click here to reset your password</a></p>
					</body>
				</html>
			`, otp, link))
		} else {
			m.SetBody("text/plain", fmt.Sprintf("%s\nYour OTP is: %s", message, otp))
			m.SetBody("text/html", fmt.Sprintf(`
				<html>
					<head>
						<meta charset="utf-8">
						<title>Your OTP Code</title>
					</head>
					<body>
						<p>Your OTP (Verification code): <strong>%s</strong></p>
					</body>
				</html>
			`, otp))
		}
	}

	// Attach file if path is provided
	if attachmentPath != "" {
		if _, err := os.Stat(attachmentPath); err == nil {
			m.Attach(attachmentPath)
			config.Logger.Debug("Attaching file to email", zap.String("filepath", attachmentPath))
		} else {
			// Use config.Logger for this non-fatal warning
			config.Logger.Warn("Attachment file not found for email",
				zap.String("filepath", attachmentPath),
				zap.String("to_email", email),
				zap.Error(err),
			)
			// Don't fail the email send just because an optional attachment isn't found
		}
	}

	// Send the email and return any error
	if err := mailer.DialAndSend(m); err != nil {
		config.Logger.Error("Failed to send email via SMTP",
			zap.String("to_email", email),
			zap.String("subject", title),
			zap.Bool("has_otp", otp != ""),
			zap.Bool("has_attachment", attachmentPath != ""),
			zap.Error(err), // Log the actual SMTP error
		)
		return fmt.Errorf("failed to send email: %w", err)
	}

	config.Logger.Info("Email sent successfully",
		zap.String("to_email", email),
		zap.String("subject", title),
		zap.Bool("has_otp", otp != ""),
	)
	return nil // return nil if email sent successfully
}
