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
