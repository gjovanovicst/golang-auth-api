package admin

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/web"
	"github.com/google/uuid"
)

// ============================================================
// OIDC Client Management GUI
// ============================================================

// OIDCClientListItem is the view model for the OIDC client list partial.
type OIDCClientListItem struct {
	ID                string
	AppID             string
	Name              string
	Description       string
	ClientID          string
	AllowedGrantTypes string
	AllowedScopes     string
	IsConfidential    bool
	IsActive          bool
	CreatedAtStr      string
	CreatedAtFull     string
}

// oidcClientListData is passed to the oidc_client_list partial.
type oidcClientListData struct {
	Clients    []OIDCClientListItem
	Page       int
	TotalPages int
	Total      int
	AppID      string
}

// oidcClientFormData is passed to the oidc_client_form partial.
type oidcClientFormData struct {
	ID                string
	AppID             string
	Name              string
	Description       string
	RedirectURIs      string
	AllowedGrantTypes string
	AllowedScopes     string
	RequireConsent    bool
	IsConfidential    bool
	PKCERequired      bool
	LogoURL           string
	LoginTheme        string
	LoginPrimaryColor string
	IsActive          bool
	Apps              []AppWithTenant
	IsEdit            bool
}

// oidcClientDeleteData is passed to the oidc_client_delete_confirm partial.
type oidcClientDeleteData struct {
	ID       string
	Name     string
	ClientID string
}

// oidcClientSecretRotatedData is passed to the oidc_client_secret_rotated partial.
type oidcClientSecretRotatedData struct {
	PlainSecret string
	ClientID    string
}

// OIDCClientsPage renders the full OIDC clients admin page.
// GET /gui/oidc-clients
func (h *GUIHandler) OIDCClientsPage(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		apps = nil // Degrade gracefully
	}

	data := web.TemplateData{
		ActivePage:    "oidc-clients",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          apps,
	}
	c.HTML(http.StatusOK, "oidc_clients", data)
}

// OIDCClientList returns the OIDC client table HTML fragment for HTMX.
// GET /gui/oidc-clients/list
func (h *GUIHandler) OIDCClientList(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 15
	appIDStr := c.Query("app_id")

	var items []OIDCClientListItem
	var total int

	if appIDStr != "" {
		appID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid application ID.</div>`)
			return
		}
		clients, err := h.OIDCService.ListClients(appID)
		if err != nil {
			c.String(http.StatusInternalServerError, `<div class="alert alert-danger">Failed to load OIDC clients.</div>`)
			return
		}
		total = len(clients)
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		for _, cl := range clients[start:end] {
			items = append(items, OIDCClientListItem{
				ID:                cl.ID.String(),
				AppID:             cl.AppID.String(),
				Name:              cl.Name,
				Description:       cl.Description,
				ClientID:          cl.ClientID,
				AllowedGrantTypes: cl.AllowedGrantTypes,
				AllowedScopes:     cl.AllowedScopes,
				IsConfidential:    cl.IsConfidential,
				IsActive:          cl.IsActive,
				CreatedAtStr:      cl.CreatedAt.Format("Jan 2, 2006"),
				CreatedAtFull:     cl.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	} else {
		// List all apps and aggregate clients
		apps, err := h.Repo.ListAllAppsWithTenantName()
		if err != nil {
			c.String(http.StatusInternalServerError, `<div class="alert alert-danger">Failed to load applications.</div>`)
			return
		}
		for _, app := range apps {
			appID := app.ID // already uuid.UUID
			clients, listErr := h.OIDCService.ListClients(appID)
			if listErr != nil {
				continue
			}
			for _, cl := range clients {
				items = append(items, OIDCClientListItem{
					ID:                cl.ID.String(),
					AppID:             cl.AppID.String(),
					Name:              cl.Name,
					Description:       cl.Description,
					ClientID:          cl.ClientID,
					AllowedGrantTypes: cl.AllowedGrantTypes,
					AllowedScopes:     cl.AllowedScopes,
					IsConfidential:    cl.IsConfidential,
					IsActive:          cl.IsActive,
					CreatedAtStr:      cl.CreatedAt.Format("Jan 2, 2006"),
					CreatedAtFull:     cl.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
		}
		total = len(items)
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > total {
			start = total
			items = nil
		} else {
			if end > total {
				end = total
			}
			items = items[start:end]
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}

	c.HTML(http.StatusOK, "oidc_client_list", oidcClientListData{
		Clients:    items,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
		AppID:      appIDStr,
	})
}

// OIDCClientCreateForm returns the empty create form HTML fragment for HTMX.
// GET /gui/oidc-clients/new
func (h *GUIHandler) OIDCClientCreateForm(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.HTML(http.StatusOK, "oidc_client_form", oidcClientFormData{
		AllowedGrantTypes: "authorization_code,refresh_token",
		AllowedScopes:     "openid profile email",
		IsConfidential:    true,
		RequireConsent:    true,
		IsActive:          true,
		LoginTheme:        "auto",
		Apps:              apps,
	})
}

// OIDCClientCreate handles creating a new OIDC client.
// POST /gui/oidc-clients
func (h *GUIHandler) OIDCClientCreate(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	appIDStr := c.PostForm("app_id")
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	redirectURIs := strings.TrimSpace(c.PostForm("redirect_uris"))
	grantTypes := strings.TrimSpace(c.PostForm("allowed_grant_types"))
	scopes := strings.TrimSpace(c.PostForm("allowed_scopes"))
	logoURL := strings.TrimSpace(c.PostForm("logo_url"))
	loginTheme := strings.TrimSpace(c.PostForm("login_theme"))
	loginPrimaryColor := strings.TrimSpace(c.PostForm("login_primary_color"))
	isConfidential := c.PostForm("is_confidential") == "true"
	pkceRequired := c.PostForm("pkce_required") == "true"
	requireConsent := c.PostForm("require_consent") == "true"

	if loginTheme == "" {
		loginTheme = "auto"
	}

	if appIDStr == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Client name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if redirectURIs == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Redirect URIs are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if grantTypes == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Allowed grant types are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if scopes == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Allowed scopes are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	client, plainSecret, err := h.OIDCService.CreateClient(appID, name, description, redirectURIs, grantTypes, scopes, requireConsent, isConfidential, pkceRequired, logoURL, loginTheme, loginPrimaryColor)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create OIDC client. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "oidcClientListRefresh")
	c.HTML(http.StatusOK, "oidc_client_secret_rotated", oidcClientSecretRotatedData{
		PlainSecret: plainSecret,
		ClientID:    client.ClientID,
	})
}

// OIDCClientEditForm returns the pre-filled edit form HTML fragment for HTMX.
// GET /gui/oidc-clients/:id/edit
func (h *GUIHandler) OIDCClientEditForm(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid client ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	client, err := h.OIDCService.GetClient(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">OIDC client not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.HTML(http.StatusOK, "oidc_client_form", oidcClientFormData{
		ID:                client.ID.String(),
		AppID:             client.AppID.String(),
		Name:              client.Name,
		Description:       client.Description,
		RedirectURIs:      client.RedirectURIs,
		AllowedGrantTypes: client.AllowedGrantTypes,
		AllowedScopes:     client.AllowedScopes,
		RequireConsent:    client.RequireConsent,
		IsConfidential:    client.IsConfidential,
		PKCERequired:      client.PKCERequired,
		LogoURL:           client.LogoURL,
		LoginTheme:        client.LoginTheme,
		LoginPrimaryColor: client.LoginPrimaryColor,
		IsActive:          client.IsActive,
		Apps:              apps,
		IsEdit:            true,
	})
}

// OIDCClientUpdate handles updating an OIDC client.
// PUT /gui/oidc-clients/:id
func (h *GUIHandler) OIDCClientUpdate(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid client ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	redirectURIs := strings.TrimSpace(c.PostForm("redirect_uris"))
	grantTypes := strings.TrimSpace(c.PostForm("allowed_grant_types"))
	scopes := strings.TrimSpace(c.PostForm("allowed_scopes"))
	logoURL := strings.TrimSpace(c.PostForm("logo_url"))
	loginTheme := strings.TrimSpace(c.PostForm("login_theme"))
	loginPrimaryColor := strings.TrimSpace(c.PostForm("login_primary_color"))

	isConfidentialVal := c.PostForm("is_confidential") == "true"
	pkceVal := c.PostForm("pkce_required") == "true"
	consentVal := c.PostForm("require_consent") == "true"
	isActiveVal := c.PostForm("is_active") == "true"

	_, err = h.OIDCService.UpdateClient(
		id,
		name,
		description,
		redirectURIs,
		grantTypes,
		scopes,
		logoURL,
		loginTheme,
		loginPrimaryColor,
		&consentVal,
		&isConfidentialVal,
		&pkceVal,
		&isActiveVal,
	)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update OIDC client. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "oidcClientListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">OIDC client updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// OIDCClientDeleteConfirm returns the delete confirmation modal body for HTMX.
// GET /gui/oidc-clients/:id/delete
func (h *GUIHandler) OIDCClientDeleteConfirm(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="modal-body"><div class="alert alert-danger">OIDC service unavailable</div></div>`)
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, `<div class="modal-body"><div class="alert alert-danger">Invalid client ID.</div></div>`)
		return
	}

	client, err := h.OIDCService.GetClient(id)
	if err != nil {
		c.String(http.StatusNotFound, `<div class="modal-body"><div class="alert alert-danger">OIDC client not found.</div></div>`)
		return
	}

	c.HTML(http.StatusOK, "oidc_client_delete_confirm", oidcClientDeleteData{
		ID:       client.ID.String(),
		Name:     client.Name,
		ClientID: client.ClientID,
	})
}

// OIDCClientDelete handles deleting an OIDC client.
// DELETE /gui/oidc-clients/:id
func (h *GUIHandler) OIDCClientDelete(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid client ID.</div>`)
		return
	}

	if err := h.OIDCService.DeleteClient(id); err != nil {
		c.String(http.StatusInternalServerError, `<div class="alert alert-danger">Failed to delete OIDC client.</div>`)
		return
	}

	c.Header("HX-Trigger", "oidcClientDeleted, oidcClientListRefresh")
	c.HTML(http.StatusOK, "oidc_client_list", oidcClientListData{
		Clients: nil,
	})
}

// OIDCClientRotateSecret rotates the client secret and returns the new secret partial.
// POST /gui/oidc-clients/:id/rotate-secret
func (h *GUIHandler) OIDCClientRotateSecret(c *gin.Context) {
	if h.OIDCService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">OIDC service unavailable</div>`)
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid client ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	client, plainSecret, err := h.OIDCService.RotateClientSecret(id)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to rotate secret. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.HTML(http.StatusOK, "oidc_client_secret_rotated", oidcClientSecretRotatedData{
		PlainSecret: plainSecret,
		ClientID:    client.ClientID,
	})
}

// OIDCClientFormCancel returns an empty response to clear the form container.
// GET /gui/oidc-clients/form-cancel
func (h *GUIHandler) OIDCClientFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}
