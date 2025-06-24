## Phase 8: Two-Factor Authentication (2FA) Implementation

This phase details the integration of Two-Factor Authentication (2FA) into the RESTful API, providing an additional layer of security for user accounts. 2FA typically involves a second verification step beyond the traditional password, such as a code from an authenticator app (TOTP) or an SMS code.

### 8.1 2FA Methods

We will primarily focus on Time-based One-Time Passwords (TOTP) using authenticator applications (e.g., Google Authenticator, Authy) due to their security and widespread adoption. SMS-based 2FA can be considered as an alternative or additional option, but TOTP is generally preferred for its independence from mobile network issues and lower cost.

### 8.2 Database Model Updates for 2FA

To support 2FA, the `User` model will need to be updated to store 2FA-related information.

**Updated `User` Model Fields:**

| Field Name      | Data Type       | Description                                       | Constraints/Notes                                |
|-----------------|-----------------|---------------------------------------------------|--------------------------------------------------|
| `TwoFAEnabled`  | `boolean`       | Indicates if 2FA is enabled for the user.         | Default to `false`                               |
| `TwoFASecret`   | `string`        | Base32 encoded secret key for TOTP.               | Encrypted storage recommended, Nullable          |
| `TwoFARecoveryCodes` | `string[]`   | List of one-time recovery codes.                  | Encrypted storage recommended, Nullable          |

**GORM Model Definition (GoLang - additions to existing `User` struct):**

```go
type User struct {
    // ... existing fields
    TwoFAEnabled       bool      `gorm:"default:false" json:"two_fa_enabled"`
    TwoFASecret        string    `json:"-"` // Stored encrypted, not exposed via JSON
    TwoFARecoveryCodes []string  `gorm:"type:text[]" json:"-"` // Stored encrypted, not exposed via JSON
}
```

### 8.3 2FA Enrollment Process

Users will need to enroll in 2FA. This process typically involves generating a secret, displaying a QR code, and verifying the setup.

**Process Flow:**
1.  **Generate 2FA Secret:** When a user initiates 2FA setup, generate a new TOTP secret key (e.g., using `github.com/pquerna/otp/totp`).
2.  **Store Secret (Temporarily):** Store this secret temporarily in Redis or a session, associated with the user.
3.  **Generate QR Code:** Create a provisioning URI and generate a QR code image from it. The URI includes the secret, user email, and issuer name.
4.  **Display QR Code:** Return the QR code image (or its URL) and the secret key to the frontend.
5.  **User Scans/Enters:** The user scans the QR code with their authenticator app or manually enters the secret.
6.  **Verify 2FA Setup:** The user enters a TOTP code from their app. The backend verifies this code against the temporarily stored secret.
7.  **Finalize Enrollment:** If verification is successful, save the `TwoFASecret` to the `User` model in the database, set `TwoFAEnabled` to `true`, and generate recovery codes. Invalidate the temporary secret in Redis/session.

**Example Endpoints:**
-   `POST /2fa/generate`: Generates a 2FA secret and QR code.
-   `POST /2fa/verify-setup`: Verifies the initial 2FA setup with a TOTP code.
-   `POST /2fa/enable`: Enables 2FA for the user after successful verification.
-   `POST /2fa/disable`: Disables 2FA for the user (requires password and/or TOTP code).
-   `GET /2fa/recovery-codes`: Generates and displays new recovery codes (requires password and/or TOTP code).

### 8.4 2FA Login Process

Once 2FA is enabled, the login process will require an additional step.

**Process Flow:**
1.  **Initial Login:** User provides email and password. Backend verifies credentials.
2.  **Check 2FA Status:** If credentials are valid and `TwoFAEnabled` is `true` for the user, the backend returns a response indicating that 2FA is required (e.g., HTTP 202 Accepted with a temporary token or session ID).
3.  **Request 2FA Code:** Frontend prompts the user for their TOTP code.
4.  **Verify 2FA Code:** User submits the TOTP code. Backend verifies the code against the stored `TwoFASecret`.
5.  **Issue JWTs:** If the TOTP code is valid, issue the access and refresh tokens.

**Example Endpoints:**
-   `POST /login`: (Modified) Handles initial email/password login. If 2FA is enabled, returns a specific status/message.
-   `POST /2fa/login-verify`: Verifies the TOTP code during login.

### 8.5 Libraries and Tools

-   **TOTP Generation/Validation:** `github.com/pquerna/otp/totp`
-   **Base32 Encoding:** Go's `encoding/base32` package.
-   **QR Code Generation:** `github.com/skip2/go-qrcode` (for generating QR code images).

### 8.6 Security Considerations

-   **Secret Storage:** The `TwoFASecret` and `TwoFARecoveryCodes` MUST be stored encrypted in the database.
-   **Rate Limiting:** Implement rate limiting on 2FA verification attempts to prevent brute-force attacks.
-   **Recovery Codes:** Ensure recovery codes are generated securely, displayed to the user only once, and handled with extreme care. They should be single-use.
-   **User Experience:** Provide clear instructions and feedback to the user throughout the 2FA enrollment and login processes.

This phase will significantly enhance the security posture of the application by adding robust Two-Factor Authentication capabilities.

