// services/email.go
package services

import (
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/utils"

	"go.uber.org/zap"
)

func (ds *DeviceService) SendDeviceRegistrationEmail(userID, email string, device *TrustedDevice) {
	emailContent := fmt.Sprintf(`
		<h2>New Device Registered</h2>
		<p>A new device has been added to your trusted devices:</p>
		<ul>
			<li><strong>Device:</strong> %s</li>
			<li><strong>Time:</strong> %s</li>
			<li><strong>IP Address:</strong> %s</li>
		</ul>
		<p>If this wasn't you, please <a href="%s/auth/devices">manage your devices</a> immediately.</p>
		<p>You can remove this device at any time from your account settings.</p>
	`,
		device.DeviceName,
		device.RegisteredAt.Format("Jan 2, 2006 at 3:04 PM"),
		device.Fingerprint.IPAddress,
		ds.frontendBaseURL)

	subject := "New Device Added to Your Account"

	utils.SendEmail(email, subject, emailContent, "", "")

	config.Logger.Info("Device registration email sent",
		zap.String("userID", userID),
		zap.String("email", email),
		zap.String("deviceID", device.DeviceID))
}
