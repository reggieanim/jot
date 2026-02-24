package httpadapter

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reggieanim/jot/internal/modules/users/app"
	"github.com/reggieanim/jot/internal/modules/users/domain"
	"github.com/reggieanim/jot/internal/platform/auth"
	"github.com/reggieanim/jot/internal/shared/errs"
	"go.uber.org/zap"
)

type Handler struct {
	service *app.Service
	jwt     *auth.JWTIssuer
	logger  *zap.Logger
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

func RegisterRoutes(router *gin.Engine, service *app.Service, jwtIssuer *auth.JWTIssuer, logger *zap.Logger) {
	h := &Handler{service: service, jwt: jwtIssuer, logger: logger}

	v1 := router.Group("/v1")

	// Public auth routes
	v1.POST("/auth/signup", h.signup)
	v1.POST("/auth/login", h.login)
	v1.POST("/auth/logout", h.logout)

	// Public profile
	v1.GET("/users/username/:username", h.getPublicProfile)

	// Protected routes
	protected := v1.Group("")
	protected.Use(auth.Middleware(jwtIssuer))
	{
		protected.GET("/auth/me", h.me)
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
	uid, _ := auth.GetUserID(c)
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

// usernameFromEmail derives a username from the email local part.
func usernameFromEmail(email string) string {
	for i, ch := range email {
		if ch == '@' {
			return email[:i]
		}
	}
	return email
}
