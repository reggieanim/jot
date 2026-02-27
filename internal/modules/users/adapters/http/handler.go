package httpadapter

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reggieanim/jot/internal/modules/users/app"
	"github.com/reggieanim/jot/internal/modules/users/domain"
	"github.com/reggieanim/jot/internal/platform/auth"
	"github.com/reggieanim/jot/internal/shared/errs"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	service     *app.Service
	jwt         *auth.JWTIssuer
	logger      *zap.Logger
	oauthCfg    *oauth2.Config
	frontendURL string
}

// --- request / response types ---

type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  domain.User `json:"user"`
}

// --- registration ---

func RegisterRoutes(router *gin.Engine, service *app.Service, jwtIssuer *auth.JWTIssuer, logger *zap.Logger, googleClientID, googleClientSecret, googleCallbackURL, frontendURL string) {
	oauthCfg := &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  googleCallbackURL,
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint:     google.Endpoint,
	}
	h := &Handler{service: service, jwt: jwtIssuer, logger: logger, oauthCfg: oauthCfg, frontendURL: frontendURL}

	v1 := router.Group("/v1")

	// Public auth routes
	v1.POST("/auth/signup", h.signup)
	v1.POST("/auth/login", h.login)
	v1.POST("/auth/logout", h.logout)
	v1.GET("/auth/me", auth.OptionalMiddleware(jwtIssuer), h.me)
	v1.GET("/auth/google", h.googleLogin)
	v1.GET("/auth/google/callback", h.googleCallback)

	// Public profile
	v1.GET("/users/username/:username", h.getPublicProfile)

	// Protected routes
	protected := v1.Group("")
	protected.Use(auth.Middleware(jwtIssuer))
	{
		protected.PUT("/auth/me", h.updateProfile)

		protected.POST("/users/:userID/follow", h.follow)
		protected.DELETE("/users/:userID/follow", h.unfollow)
		protected.GET("/users/:userID/followers", h.listFollowers)
		protected.GET("/users/:userID/following", h.listFollowing)
		protected.GET("/users/:userID/is-following", h.isFollowing)
	}
}

// --- handlers ---

func (h *Handler) signup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Use name as both display name and derive username from email prefix.
	username := usernameFromEmail(req.Email)
	user, token, err := h.service.Signup(c.Request.Context(), req.Email, username, req.Name, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}

	setTokenCookie(c, token)
	c.JSON(http.StatusCreated, authResponse{Token: token, User: user})
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, token, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}

	setTokenCookie(c, token)
	c.JSON(http.StatusOK, authResponse{Token: token, User: user})
}

func (h *Handler) me(c *gin.Context) {
	uid, exists := auth.GetUserID(c)
	if !exists {
		c.Status(http.StatusNoContent)
		return
	}
	user, err := h.service.GetProfile(c.Request.Context(), uid)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) updateProfile(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.service.UpdateProfile(c.Request.Context(), uid, req.DisplayName, req.Bio, req.AvatarURL); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) getPublicProfile(c *gin.Context) {
	username := c.Param("username")
	profile, err := h.service.GetPublicProfile(c.Request.Context(), username)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) follow(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	followeeID := domain.UserID(c.Param("userID"))
	if err := h.service.Follow(c.Request.Context(), uid, followeeID); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) unfollow(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	followeeID := domain.UserID(c.Param("userID"))
	if err := h.service.Unfollow(c.Request.Context(), uid, followeeID); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listFollowers(c *gin.Context) {
	targetID := domain.UserID(c.Param("userID"))
	profiles, err := h.service.ListFollowers(c.Request.Context(), targetID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profiles)
}

func (h *Handler) listFollowing(c *gin.Context) {
	targetID := domain.UserID(c.Param("userID"))
	profiles, err := h.service.ListFollowing(c.Request.Context(), targetID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profiles)
}

func (h *Handler) isFollowing(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	targetID := domain.UserID(c.Param("userID"))
	following, err := h.service.IsFollowing(c.Request.Context(), uid, targetID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"following": following})
}

// --- helpers ---

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, errs.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, errs.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict"})
	default:
		h.logger.Error("internal error", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func setTokenCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("jot_token", token, int((7 * 24 * time.Hour).Seconds()), "/", "", false, true)
}

func (h *Handler) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("jot_token", "", -1, "/", "", false, true)
	c.Status(http.StatusNoContent)
}

// googleLogin redirects the browser to Google's OAuth consent screen.
func (h *Handler) googleLogin(c *gin.Context) {
	state := randomState()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	url := h.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
	c.Redirect(http.StatusFound, url)
}

// googleCallback handles the redirect from Google after the user consents.
func (h *Handler) googleCallback(c *gin.Context) {
	// Validate CSRF state.
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie != c.Query("state") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
		return
	}
	// Clear state cookie.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	// Exchange code for token.
	token, err := h.oauthCfg.Exchange(context.Background(), c.Query("code"))
	if err != nil {
		h.logger.Error("google oauth exchange", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "code exchange failed"})
		return
	}

	// Fetch user info from Google.
	client := h.oauthCfg.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		h.logger.Error("google userinfo", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user info"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var info struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &info); err != nil || info.Email == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user info from google"})
		return
	}

	user, jwtToken, err := h.service.LoginOrSignupWithGoogle(c.Request.Context(), info.Email, info.Name, info.Picture)
	if err != nil {
		h.handleError(c, err)
		return
	}

	setTokenCookie(c, jwtToken)
	_ = user
	c.Redirect(http.StatusFound, h.frontendURL)
}

// randomState returns a short random string for CSRF protection.
func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// usernameFromEmail derives a username from the email local part.
func usernameFromEmail(email string) string {
	for i, ch := range email {
		if ch == '@' {
			return email[:i]
		}
	}
	return email
}
