package email

import "github.com/gjovanovicst/auth_api/pkg/models"

// GetDefaultTemplate returns the hardcoded fallback template for a given email type.
// These are used when neither an app-specific nor a global DB template exists.
func GetDefaultTemplate(typeCode string) *models.EmailTemplate {
	switch typeCode {
	case TypeEmailVerification:
		return defaultEmailVerification()
	case TypePasswordReset:
		return defaultPasswordReset()
	case TypeTwoFACode:
		return defaultTwoFACode()
	case TypeWelcome:
		return defaultWelcome()
	case TypeAccountDeactivated:
		return defaultAccountDeactivated()
	case TypePasswordChanged:
		return defaultPasswordChanged()
	case TypeMagicLink:
		return defaultMagicLink()
	case TypeNewDeviceLogin:
		return defaultNewDeviceLogin()
	case TypeSuspiciousActivity:
		return defaultSuspiciousActivity()
	case TypeApiKeyExpiringSoon:
		return defaultApiKeyExpiringSoon()
	default:
		return nil
	}
}

func defaultEmailVerification() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Email Verification",
		Subject:        "Verify Your Email Address",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Verify Your Email</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Verify Your Email Address</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      Thank you for registering. Please click the button below to verify your email address and activate your account.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.VerificationLink}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Verify Email Address</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn't work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.VerificationLink}}</p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you did not create an account, you can safely ignore this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Verify Your Email Address

Thank you for registering with {{.AppName}}.

Please verify your email address by clicking the link below:
{{.VerificationLink}}

If you did not create an account, you can safely ignore this email.`,
	}
}

func defaultPasswordReset() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Password Reset",
		Subject:        "Reset Your Password",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Reset Your Password</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Reset Your Password</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We received a request to reset your password. Click the button below to choose a new password.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.ResetLink}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Reset Password</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn't work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.ResetLink}}</p>
    <p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">
      This link will expire in {{.ExpirationMinutes}} minutes.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you didn't request a password reset, you can safely ignore this email. Your password will not be changed.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Reset Your Password

We received a request to reset your password for your {{.AppName}} account.

Click the link below to reset your password:
{{.ResetLink}}

This link will expire in {{.ExpirationMinutes}} minutes.

If you didn't request a password reset, you can safely ignore this email.`,
	}
}

func defaultTwoFACode() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default 2FA Verification Code",
		Subject:        "Your Verification Code",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Your Verification Code</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;text-align:center;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Your Verification Code</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 32px;">
      Use the following code to complete your sign-in. This code is valid for {{.ExpirationMinutes}} minutes.
    </p>
    <div style="background-color:#f0f4ff;border:2px solid #4f46e5;border-radius:12px;padding:24px;display:inline-block;margin:0 0 32px;">
      <span style="font-size:36px;font-weight:700;letter-spacing:8px;color:#1a1a2e;font-family:'Courier New',monospace;">{{.Code}}</span>
    </div>
    <p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">
      Do not share this code with anyone. Our team will never ask for your code.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you did not request this code, someone may be trying to access your account. Please change your password immediately.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Your Verification Code

Use the following code to complete your sign-in to {{.AppName}}:

{{.Code}}

This code is valid for {{.ExpirationMinutes}} minutes.

Do not share this code with anyone. Our team will never ask for your code.

If you did not request this code, please change your password immediately.`,
	}
}

func defaultWelcome() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Welcome Email",
		Subject:        "Welcome to {{.AppName}}",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Welcome</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Welcome!</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      Your email has been verified and your account is now active. Welcome to {{.AppName}}!
    </p>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      Here are a few things you can do to get started:
    </p>
    <ul style="color:#4a5568;font-size:16px;line-height:1.8;margin:0 0 24px;padding-left:24px;">
      <li>Complete your profile information</li>
      <li>Set up two-factor authentication for added security</li>
      <li>Explore the features available to you</li>
    </ul>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you have any questions, please contact our support team.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Welcome to {{.AppName}}!

Your email has been verified and your account is now active.

Here are a few things you can do to get started:
- Complete your profile information
- Set up two-factor authentication for added security
- Explore the features available to you

If you have any questions, please contact our support team.`,
	}
}

func defaultAccountDeactivated() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Account Deactivated",
		Subject:        "Your Account Has Been Deactivated",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Account Deactivated</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#e53e3e;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Account Deactivated</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      Your account on {{.AppName}} has been deactivated. You will no longer be able to sign in or access your account.
    </p>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      If you believe this was done in error, please contact the application administrator to have your account reactivated.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      This is an automated notification. Please do not reply to this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Account Deactivated

Your account on {{.AppName}} has been deactivated. You will no longer be able to sign in or access your account.

If you believe this was done in error, please contact the application administrator to have your account reactivated.`,
	}
}

func defaultPasswordChanged() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Password Changed",
		Subject:        "Your Password Has Been Changed",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Password Changed</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#f6ad55;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Password Changed</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      Your password for {{.AppName}} was successfully changed on {{.ChangeTime}}.
    </p>
    <p style="color:#e53e3e;font-size:16px;line-height:1.6;margin:0 0 24px;font-weight:600;">
      If you did not make this change, please reset your password immediately and contact support.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      This is a security notification. Please do not reply to this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Password Changed

Your password for {{.AppName}} was successfully changed on {{.ChangeTime}}.

If you did not make this change, please reset your password immediately and contact support.

This is a security notification.`,
	}
}

func defaultMagicLink() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Magic Link Login",
		Subject:        "Sign In to Your Account",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Sign In to Your Account</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Sign In to Your Account</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We received a request to sign in to your account. Click the button below to log in instantly — no password needed.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.MagicLink}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Sign In Now</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn't work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.MagicLink}}</p>
    <p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">
      This link will expire in {{.ExpirationMinutes}} minutes and can only be used once.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you didn't request this link, you can safely ignore this email. No one can access your account without clicking the link above.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Sign In to Your Account

We received a request to sign in to your {{.AppName}} account.

Click the link below to log in instantly:
{{.MagicLink}}

This link will expire in {{.ExpirationMinutes}} minutes and can only be used once.

If you didn't request this link, you can safely ignore this email.`,
	}
}

func defaultNewDeviceLogin() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default New Device Login Notification",
		Subject:        "New Login to Your Account",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>New Login Detected</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#3182ce;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">New Login Detected</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We noticed a new login to your account from a device or location we haven't seen before:
    </p>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f7fafc;border-radius:8px;border:1px solid #e2e8f0;margin:0 0 24px;">
      <tr><td style="padding:20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="4">
          <tr>
            <td style="color:#718096;font-size:14px;width:100px;vertical-align:top;padding:4px 8px;">IP Address:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginIP}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Location:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginLocation}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Device:</td>
            <td style="color:#1a1a2e;font-size:14px;padding:4px 8px;">{{.LoginDevice}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Time:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginTime}}</td>
          </tr>
        </table>
      </td></tr>
    </table>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 16px;">
      If this was you, you can ignore this email. If you don't recognize this activity, we recommend changing your password immediately.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      This is an automated security notification from {{.AppName}}.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `New Login Detected

We noticed a new login to your {{.AppName}} account:

IP Address: {{.LoginIP}}
Location:   {{.LoginLocation}}
Device:     {{.LoginDevice}}
Time:       {{.LoginTime}}

If this was you, you can ignore this email. If you don't recognize this activity, we recommend changing your password immediately.

This is an automated security notification from {{.AppName}}.`,
	}
}

func defaultApiKeyExpiringSoon() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default API Key Expiring Soon",
		Subject:        "API Key '{{.ApiKeyName}}' expires in {{.DaysUntilExpiry}} days",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>API Key Expiring Soon</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">
      <span style="color:#d97706;">&#9888;</span> API Key Expiring Soon
    </h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      An API key in your <strong>{{.AppName}}</strong> account will expire in
      <strong>{{.DaysUntilExpiry}} day{{if ne .DaysUntilExpiry "1"}}s{{end}}</strong>.
      Please rotate this key before it expires to avoid service interruption.
    </p>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f8fafc;border-radius:6px;padding:20px;margin:0 0 24px;">
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;width:140px;">Key Name:</td>
        <td style="padding:6px 0;color:#1e293b;font-size:14px;font-weight:600;">{{.ApiKeyName}}</td>
      </tr>
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;">Key Identifier:</td>
        <td style="padding:6px 0;color:#1e293b;font-size:14px;font-family:monospace;">{{.ApiKeyPrefix}}...</td>
      </tr>
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;">Key Type:</td>
        <td style="padding:6px 0;color:#1e293b;font-size:14px;text-transform:capitalize;">{{.ApiKeyType}}</td>
      </tr>
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;">Expires At:</td>
        <td style="padding:6px 0;color:#dc2626;font-size:14px;font-weight:600;">{{.ApiKeyExpiresAt}}</td>
      </tr>
    </table>
    <p style="color:#4a5568;font-size:15px;line-height:1.6;margin:0 0 24px;">
      To rotate this key, log in to the admin panel and generate a new API key, then update your integrations before the expiry date.
    </p>
    <p style="color:#9ca3af;font-size:13px;line-height:1.6;margin:0;">
      This is an automated notification from {{.AppName}}. If you did not expect this email or have already rotated this key, you can safely ignore it.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:20px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#94a3b8;font-size:12px;margin:0;">
      &copy; {{.AppName}} &mdash; Automated Security Notification
    </p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `API Key Expiring Soon

An API key in your {{.AppName}} account will expire in {{.DaysUntilExpiry}} day(s).

Key Name:       {{.ApiKeyName}}
Key Identifier: {{.ApiKeyPrefix}}...
Key Type:       {{.ApiKeyType}}
Expires At:     {{.ApiKeyExpiresAt}}

Please rotate this key before it expires to avoid service interruption.
Log in to the admin panel and generate a new API key, then update your integrations.

This is an automated notification from {{.AppName}}.`,
	}
}

func defaultSuspiciousActivity() *models.EmailTemplate {
	return &models.EmailTemplate{
		Name:           "Default Suspicious Activity Alert",
		Subject:        "Security Alert: Suspicious Activity on Your Account",
		TemplateEngine: models.TemplateEngineGoTemplate,
		BodyHTML: `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Security Alert</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#e53e3e;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Security Alert</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We detected suspicious activity on your account. Please review the details below:
    </p>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#fff5f5;border-radius:8px;border:1px solid #feb2b2;margin:0 0 24px;">
      <tr><td style="padding:20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="4">
          <tr>
            <td style="color:#718096;font-size:14px;width:100px;vertical-align:top;padding:4px 8px;">Alert:</td>
            <td style="color:#e53e3e;font-size:14px;font-weight:600;padding:4px 8px;">{{.AlertDetails}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">IP Address:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginIP}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Location:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginLocation}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Device:</td>
            <td style="color:#1a1a2e;font-size:14px;padding:4px 8px;">{{.LoginDevice}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Time:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginTime}}</td>
          </tr>
        </table>
      </td></tr>
    </table>
    <p style="color:#e53e3e;font-size:16px;line-height:1.6;margin:0 0 16px;font-weight:600;">
      If you don't recognize this activity, we strongly recommend:
    </p>
    <ul style="color:#4a5568;font-size:16px;line-height:1.8;margin:0 0 24px;padding-left:24px;">
      <li>Changing your password immediately</li>
      <li>Enabling two-factor authentication if not already active</li>
      <li>Reviewing your recent account activity</li>
    </ul>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      This is an automated security alert from {{.AppName}}.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`,
		BodyText: `Security Alert: Suspicious Activity on Your Account

We detected suspicious activity on your {{.AppName}} account:

Alert:      {{.AlertDetails}}
IP Address: {{.LoginIP}}
Location:   {{.LoginLocation}}
Device:     {{.LoginDevice}}
Time:       {{.LoginTime}}

If you don't recognize this activity, we strongly recommend:
- Changing your password immediately
- Enabling two-factor authentication if not already active
- Reviewing your recent account activity

This is an automated security alert from {{.AppName}}.`,
	}
}
