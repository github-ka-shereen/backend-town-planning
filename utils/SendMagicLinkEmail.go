package utils

import (
	"fmt"
	"os"
	"town-planning-backend/config"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

// SendMagicLinkEmail sends a styled magic link email
func SendMagicLinkEmail(email string, magicLinkURL string, expiresIn string) error {
	mailer := GetMailer()
	if mailer == nil {
		err := fmt.Errorf("mailer is not initialized")
		config.Logger.Error("Email send failed: mailer is not initialized",
			zap.String("to_email", email),
			zap.String("subject", "Your Magic Link"),
			zap.Error(err),
		)
		return err
	}

	from := os.Getenv("SMTP_FROM")
	if from == "" {
		err := fmt.Errorf("SMTP_FROM environment variable not set")
		config.Logger.Error("Email send failed: SMTP_FROM not set",
			zap.String("to_email", email),
			zap.Error(err),
		)
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Your Magic Link")

	// Plain text version
	plainText := fmt.Sprintf(
		"Click the link below to sign in:\n\n%s\n\nThis link expires in %s.\n\nIf you did not request this link, you can safely ignore this email.",
		magicLinkURL,
		expiresIn,
	)

	// HTML version with styling
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Your Magic Link</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            text-align: center;
            padding: 10px 0;
            border-bottom: 1px solid #eee;
            margin-bottom: 30px;
        }
        .button {
            display: block;
            width: 200px;
            margin: 30px auto;
            padding: 12px 24px;
            background-color: #4F46E5;
            color: white !important;
            text-align: center;
            text-decoration: none;
            border-radius: 6px;
            font-weight: bold;
            font-size: 16px;
        }
        .button:hover {
            background-color: #4338CA;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            text-align: center;
            color: #777;
            font-size: 0.9em;
        }
        .note {
            background-color: #f9f9f9;
            padding: 15px;
            border-radius: 6px;
            margin: 20px 0;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="header">
        <h2>Your Magic Link</h2>
    </div>
    
    <p>Click the button below to securely sign in to your account:</p>
    
    <a href="%s" class="button">Sign In Now</a>
    
    <div class="note">
        <p><strong>This link expires in %s.</strong> For security reasons, please don't share this link with anyone.</p>
    </div>
    
    <p>If you didn't request this link, you can safely ignore this email.</p>
    
    <div class="footer">
        <p>Sent by AcrePoint from Our Team</p>
    </div>
</body>
</html>
`, magicLinkURL, expiresIn)

	m.SetBody("text/plain", plainText)
	m.AddAlternative("text/html", htmlBody)

	if err := mailer.DialAndSend(m); err != nil {
		config.Logger.Error("Failed to send magic link email",
			zap.String("to_email", email),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send magic link email: %w", err)
	}

	config.Logger.Info("Magic link email sent successfully",
		zap.String("to_email", email),
	)
	return nil
}
