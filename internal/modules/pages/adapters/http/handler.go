package httpadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	jnats "github.com/nats-io/nats.go"
	"github.com/reggieanim/jot/internal/modules/pages/app"
	"github.com/reggieanim/jot/internal/modules/pages/domain"
	"github.com/reggieanim/jot/internal/platform/auth"
	"github.com/reggieanim/jot/internal/platform/storage"
	"github.com/reggieanim/jot/internal/shared/errs"
	"go.uber.org/zap"
)

type Handler struct {
	service *app.Service
	logger  *zap.Logger
	conn    *jnats.Conn
	subject string
	media   storage.MediaStore
}

type pageEvent struct {
	Type      string      `json:"type"`
	Page      domain.Page `json:"page"`
	Timestamp time.Time   `json:"timestamp"`
}

type typingPresence struct {
	PageID    string `json:"page_id"`
	BlockID   string `json:"block_id"`
	SessionID string `json:"session_id"`
	UserName  string `json:"user_name"`
	IsTyping  bool   `json:"is_typing"`
}

type pagePresence struct {
	PageID    string `json:"page_id"`
	SessionID string `json:"session_id"`
	UserName  string `json:"user_name"`
	IsOnline  bool   `json:"is_online"`
}

type streamEvent struct {
	Type      string          `json:"type"`
	Page      *domain.Page    `json:"page,omitempty"`
	Typing    *typingPresence `json:"typing,omitempty"`
	Presence  *pagePresence   `json:"presence,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type createPageRequest struct {
	Title     string         `json:"title"`
	Cover     *string        `json:"cover,omitempty"`
	Blocks    []domain.Block `json:"blocks"`
	DarkMode  bool           `json:"dark_mode"`
	Cinematic bool           `json:"cinematic"`
	Mood      int            `json:"mood"`
	BgColor   string         `json:"bg_color"`
}

type updateBlocksRequest struct {
	Blocks []domain.Block `json:"blocks"`
}

type updateBlocksRealtimeRequest struct {
	Blocks        []domain.Block `json:"blocks"`
	BaseUpdatedAt *string        `json:"base_updated_at,omitempty"`
}

type updatePageMetaRequest struct {
	Title         string  `json:"title"`
	Cover         *string `json:"cover,omitempty"`
	DarkMode      bool    `json:"dark_mode"`
	Cinematic     bool    `json:"cinematic"`
	Mood          int     `json:"mood"`
	BgColor       string  `json:"bg_color"`
	BaseUpdatedAt *string `json:"base_updated_at,omitempty"`
}

type publishPageRequest struct {
	Published bool `json:"published"`
}

type createProofreadRequest struct {
	AuthorName  string                       `json:"author_name"`
	Title       string                       `json:"title"`
	Summary     string                       `json:"summary"`
	Stance      string                       `json:"stance"`
	Annotations []domain.ProofreadAnnotation `json:"annotations"`
}

type publishTypingRequest struct {
	BlockID   string `json:"block_id"`
	SessionID string `json:"session_id"`
	UserName  string `json:"user_name"`
	IsTyping  bool   `json:"is_typing"`
}

type publishPresenceRequest struct {
	SessionID string `json:"session_id"`
	UserName  string `json:"user_name"`
	IsOnline  bool   `json:"is_online"`
}

func RegisterRoutes(router *gin.Engine, service *app.Service, conn *jnats.Conn, subject string, logger *zap.Logger, media storage.MediaStore, jwtIssuer *auth.JWTIssuer) {
	handler := &Handler{service: service, logger: logger, conn: conn, subject: subject, media: media}
	v1 := router.Group("/v1")

	// Public endpoints (no auth required)
	v1.GET("/public/pages/:pageID", handler.getPublicPage)
	v1.GET("/public/pages/:pageID/blocks/:blockID", handler.getPublicBlock)
	v1.GET("/public/pages/:pageID/proofreads", handler.listProofreads)
	v1.POST("/public/pages/:pageID/proofreads", handler.createProofread)
	v1.GET("/public/proofreads/:proofreadID", handler.getProofread)
	v1.GET("/users/:userID/pages", handler.listPublishedPagesByUser)
	v1.GET("/public/feed", handler.listFeed)

	// SSE + realtime (EventSource can't send cookies/headers)
	v1.GET("/pages/:pageID/events", handler.subscribePageEvents)

	// Protected endpoints (require auth)
	protected := v1.Group("")
	protected.Use(auth.Middleware(jwtIssuer))
	{
		protected.POST("/media/images", handler.uploadImage)
		protected.POST("/pages/:pageID/presence", handler.publishPresence)
		protected.POST("/pages/:pageID/typing", handler.publishTyping)
		protected.POST("/pages", handler.createPage)
		protected.GET("/pages", handler.listPages)
		protected.GET("/pages/archived", handler.listArchivedPages)
		protected.GET("/pages/:pageID", handler.getPage)
		protected.DELETE("/pages/:pageID", handler.deletePage)
		protected.PUT("/pages/:pageID/archive", handler.archivePage)
		protected.PUT("/pages/:pageID/restore", handler.restorePage)
		protected.PUT("/pages/:pageID/blocks", handler.updateBlocks)
		protected.PUT("/pages/:pageID/realtime-blocks", handler.updateBlocksRealtime)
		protected.PUT("/pages/:pageID/meta", handler.updatePageMeta)
		protected.PUT("/pages/:pageID/publish", handler.setPagePublished)
	}
}

func (handler *Handler) listPages(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pages, err := handler.service.ListPages(ctx.Request.Context(), string(uid))
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": pages})
}

func (handler *Handler) deletePage(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	if err := handler.service.DeletePage(ctx.Request.Context(), string(uid), pageID); err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"status": "deleted"})
}

func (handler *Handler) archivePage(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	if err := handler.service.ArchivePage(ctx.Request.Context(), string(uid), pageID); err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"status": "archived"})
}

func (handler *Handler) restorePage(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	if err := handler.service.RestorePage(ctx.Request.Context(), string(uid), pageID); err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"status": "restored"})
}

func (handler *Handler) listArchivedPages(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pages, err := handler.service.ListArchivedPages(ctx.Request.Context(), string(uid))
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": pages})
}

func (handler *Handler) setPagePublished(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	var body publishPageRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	page, err := handler.service.SetPagePublished(ctx.Request.Context(), string(uid), pageID, body.Published)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}

	ctx.JSON(200, gin.H{"status": "updated", "page": page})
}

func (handler *Handler) getPublicPage(ctx *gin.Context) {
	pageID := domain.PageID(ctx.Param("pageID"))
	page, err := handler.service.GetPublicPage(ctx.Request.Context(), pageID)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	readerKey := makeOrganicReaderKey(ctx)
	if unique, err := handler.service.RecordPublicRead(ctx.Request.Context(), pageID, readerKey); err != nil {
		handler.logger.Warn("record organic read failed", zap.Error(err), zap.String("page_id", string(pageID)))
	} else if unique {
		page.ReadCount++
	}
	ctx.JSON(200, page)
}

func makeOrganicReaderKey(ctx *gin.Context) string {
	ip := strings.TrimSpace(ctx.ClientIP())
	ua := strings.TrimSpace(ctx.GetHeader("User-Agent"))
	if ip == "" && ua == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(sum[:])
}

func (handler *Handler) getPublicBlock(ctx *gin.Context) {
	pageID := domain.PageID(ctx.Param("pageID"))
	blockID := ctx.Param("blockID")
	block, page, err := handler.service.GetPublicBlock(ctx.Request.Context(), pageID, blockID)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{
		"block": block,
		"page": gin.H{
			"id":        page.ID,
			"title":     page.Title,
			"cover":     page.Cover,
			"dark_mode": page.DarkMode,
			"cinematic": page.Cinematic,
			"mood":      page.Mood,
			"bg_color":  page.BgColor,
		},
	})
}

func (handler *Handler) createProofread(ctx *gin.Context) {
	pageID := domain.PageID(ctx.Param("pageID"))
	var body createProofreadRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	proofread, err := handler.service.CreateProofread(
		ctx.Request.Context(),
		pageID,
		body.AuthorName,
		body.Title,
		body.Summary,
		body.Stance,
		body.Annotations,
	)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}

	ctx.JSON(201, proofread)
}

func (handler *Handler) listProofreads(ctx *gin.Context) {
	pageID := domain.PageID(ctx.Param("pageID"))
	proofreads, err := handler.service.ListProofreads(ctx.Request.Context(), pageID)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": proofreads})
}

func (handler *Handler) getProofread(ctx *gin.Context) {
	proofreadID := domain.ProofreadID(ctx.Param("proofreadID"))
	proofread, page, err := handler.service.GetProofread(ctx.Request.Context(), proofreadID)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"proofread": proofread, "page": page})
}

func (handler *Handler) publishPresence(ctx *gin.Context) {
	pageID := strings.TrimSpace(ctx.Param("pageID"))
	if pageID == "" {
		ctx.JSON(400, gin.H{"error": "pageID is required"})
		return
	}
	if handler.conn == nil {
		ctx.JSON(503, gin.H{"error": "realtime unavailable"})
		return
	}

	var body publishPresenceRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	body.SessionID = strings.TrimSpace(body.SessionID)
	body.UserName = strings.TrimSpace(body.UserName)
	if body.SessionID == "" || body.UserName == "" {
		ctx.JSON(400, gin.H{"error": "session_id and user_name are required"})
		return
	}

	event := streamEvent{
		Type: "page.presence",
		Presence: &pagePresence{
			PageID:    pageID,
			SessionID: body.SessionID,
			UserName:  body.UserName,
			IsOnline:  body.IsOnline,
		},
		Timestamp: time.Now().UTC(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		ctx.JSON(500, gin.H{"error": "could not publish presence"})
		return
	}

	if err := handler.conn.Publish(handler.subject, payload); err != nil {
		handler.logger.Warn("publish presence failed", zap.Error(err))
		ctx.JSON(503, gin.H{"error": "realtime unavailable"})
		return
	}

	ctx.JSON(202, gin.H{"status": "accepted"})
}

func (handler *Handler) publishTyping(ctx *gin.Context) {
	pageID := strings.TrimSpace(ctx.Param("pageID"))
	if pageID == "" {
		ctx.JSON(400, gin.H{"error": "pageID is required"})
		return
	}
	if handler.conn == nil {
		ctx.JSON(503, gin.H{"error": "realtime unavailable"})
		return
	}

	var body publishTypingRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	body.BlockID = strings.TrimSpace(body.BlockID)
	body.SessionID = strings.TrimSpace(body.SessionID)
	body.UserName = strings.TrimSpace(body.UserName)
	if body.BlockID == "" || body.SessionID == "" || body.UserName == "" {
		ctx.JSON(400, gin.H{"error": "block_id, session_id and user_name are required"})
		return
	}

	event := streamEvent{
		Type: "page.typing",
		Typing: &typingPresence{
			PageID:    pageID,
			BlockID:   body.BlockID,
			SessionID: body.SessionID,
			UserName:  body.UserName,
			IsTyping:  body.IsTyping,
		},
		Timestamp: time.Now().UTC(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		ctx.JSON(500, gin.H{"error": "could not publish typing"})
		return
	}

	if err := handler.conn.Publish(handler.subject, payload); err != nil {
		handler.logger.Warn("publish typing failed", zap.Error(err))
		ctx.JSON(503, gin.H{"error": "realtime unavailable"})
		return
	}

	ctx.JSON(202, gin.H{"status": "accepted"})
}

func (handler *Handler) uploadImage(ctx *gin.Context) {
	const maxUploadSize = 15 << 20

	if handler.media == nil {
		ctx.JSON(503, gin.H{"error": "media storage unavailable"})
		return
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(400, gin.H{"error": "file is required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid file"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "could not read file"})
		return
	}
	if len(content) > maxUploadSize {
		ctx.JSON(413, gin.H{"error": "image too large (max 15MB)"})
		return
	}
	if len(content) == 0 {
		ctx.JSON(400, gin.H{"error": "empty file"})
		return
	}

	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	if !strings.HasPrefix(contentType, "image/") {
		ctx.JSON(400, gin.H{"error": "only image uploads are allowed"})
		return
	}

	url, key, err := handler.media.UploadImage(ctx.Request.Context(), fileHeader.Filename, contentType, content)
	if err != nil {
		handler.logger.Warn("upload image failed", zap.Error(err))
		ctx.JSON(500, gin.H{"error": "upload failed"})
		return
	}

	ctx.JSON(201, gin.H{"url": url, "key": key})
}

func (handler *Handler) subscribePageEvents(ctx *gin.Context) {
	pageID := ctx.Param("pageID")
	if pageID == "" {
		ctx.JSON(400, gin.H{"error": "pageID is required"})
		return
	}

	if handler.conn == nil {
		ctx.JSON(503, gin.H{"error": "realtime unavailable"})
		return
	}

	subscription, err := handler.conn.SubscribeSync(handler.subject)
	if err != nil {
		handler.logger.Warn("subscribe nats failed", zap.Error(err))
		ctx.JSON(503, gin.H{"error": "realtime unavailable"})
		return
	}
	defer subscription.Unsubscribe()

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Status(http.StatusOK)

	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		ctx.JSON(500, gin.H{"error": "streaming not supported"})
		return
	}

	for {
		if ctx.Request.Context().Err() != nil {
			return
		}

		msg, err := subscription.NextMsg(15 * time.Second)
		if err != nil {
			if errors.Is(err, jnats.ErrTimeout) {
				_, _ = fmt.Fprint(ctx.Writer, ": keepalive\n\n")
				flusher.Flush()
				continue
			}
			handler.logger.Warn("stream nats failed", zap.Error(err))
			return
		}

		var event streamEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			var legacy pageEvent
			if legacyErr := json.Unmarshal(msg.Data, &legacy); legacyErr != nil {
				handler.logger.Warn("invalid page event payload", zap.Error(err))
				continue
			}
			event = streamEvent{
				Type:      legacy.Type,
				Page:      &legacy.Page,
				Timestamp: legacy.Timestamp,
			}
		}

		eventName := "page"
		switch {
		case strings.HasPrefix(event.Type, "page.") && event.Type != "page.typing" && event.Type != "page.presence":
			eventName = "page"
		case event.Type == "page.typing":
			eventName = "typing"
		case event.Type == "page.presence":
			eventName = "presence"
		default:
			continue
		}

		if eventName == "page" {
			if event.Page == nil || string(event.Page.ID) != pageID {
				continue
			}
		} else if eventName == "typing" {
			if event.Typing == nil || event.Typing.PageID != pageID {
				continue
			}
		} else {
			if event.Presence == nil || event.Presence.PageID != pageID {
				continue
			}
		}

		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now().UTC()
		}

		payload, err := json.Marshal(event)
		if err != nil {
			continue
		}

		if _, err := fmt.Fprintf(ctx.Writer, "event: %s\ndata: %s\n\n", eventName, payload); err != nil {
			return
		}
		flusher.Flush()
	}
}

func (handler *Handler) createPage(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	var body createPageRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	page, err := handler.service.CreatePageWithSettings(
		ctx.Request.Context(),
		string(uid),
		body.Title,
		body.Cover,
		body.Blocks,
		body.DarkMode,
		body.Cinematic,
		body.Mood,
		body.BgColor,
	)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}

	ctx.JSON(201, page)
}

func (handler *Handler) getPage(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	page, err := handler.service.GetPage(ctx.Request.Context(), pageID)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	if page.OwnerID == nil || *page.OwnerID != string(uid) {
		ctx.JSON(403, gin.H{"error": "forbidden"})
		return
	}

	ctx.JSON(200, page)
}

func (handler *Handler) updateBlocks(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	var body updateBlocksRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	if err := handler.service.UpdateBlocks(ctx.Request.Context(), string(uid), pageID, body.Blocks); err != nil {
		handler.handleError(ctx, err)
		return
	}

	ctx.JSON(200, gin.H{"status": "updated"})
}

func (handler *Handler) updateBlocksRealtime(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	var body updateBlocksRealtimeRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	var expectedUpdatedAt *time.Time
	if body.BaseUpdatedAt != nil && *body.BaseUpdatedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *body.BaseUpdatedAt)
		if err != nil {
			ctx.JSON(400, gin.H{"error": "base_updated_at must be RFC3339Nano"})
			return
		}
		expectedUpdatedAt = &parsed
	}

	page, err := handler.service.UpdateBlocksRealtime(ctx.Request.Context(), string(uid), pageID, body.Blocks, expectedUpdatedAt)
	if err != nil {
		if errors.Is(err, errs.ErrConflict) {
			latest, getErr := handler.service.GetPage(ctx.Request.Context(), pageID)
			if getErr != nil {
				handler.handleError(ctx, getErr)
				return
			}
			ctx.JSON(409, gin.H{"error": "conflict", "conflict": true, "page": latest})
			return
		}
		handler.handleError(ctx, err)
		return
	}

	ctx.JSON(200, gin.H{"status": "updated", "page": page})
}

func (handler *Handler) updatePageMeta(ctx *gin.Context) {
	uid, _ := auth.GetUserID(ctx)
	pageID := domain.PageID(ctx.Param("pageID"))
	var body updatePageMetaRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json body"})
		return
	}

	var expectedUpdatedAt *time.Time
	if body.BaseUpdatedAt != nil && *body.BaseUpdatedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *body.BaseUpdatedAt)
		if err != nil {
			ctx.JSON(400, gin.H{"error": "base_updated_at must be RFC3339Nano"})
			return
		}
		expectedUpdatedAt = &parsed
	}

	page, err := handler.service.UpdatePageMetaRealtime(ctx.Request.Context(), string(uid), pageID, body.Title, body.Cover, body.DarkMode, body.Cinematic, body.Mood, body.BgColor, expectedUpdatedAt)
	if err != nil {
		if errors.Is(err, errs.ErrConflict) {
			latest, getErr := handler.service.GetPage(ctx.Request.Context(), pageID)
			if getErr != nil {
				handler.handleError(ctx, getErr)
				return
			}
			ctx.JSON(409, gin.H{"error": "conflict", "conflict": true, "page": latest})
			return
		}
		handler.handleError(ctx, err)
		return
	}

	ctx.JSON(200, gin.H{"status": "updated", "page": page})
}

func (handler *Handler) listFeed(ctx *gin.Context) {
	limit := 20
	offset := 0
	if l := ctx.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := ctx.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	sort := ctx.DefaultQuery("sort", "new")
	pages, err := handler.service.ListPublishedFeed(ctx.Request.Context(), limit, offset, sort)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": pages})
}

func (handler *Handler) listPublishedPagesByUser(ctx *gin.Context) {
	userID := ctx.Param("userID")
	if userID == "" {
		ctx.JSON(400, gin.H{"error": "userID is required"})
		return
	}
	pages, err := handler.service.ListPublishedPagesByOwner(ctx.Request.Context(), userID)
	if err != nil {
		handler.handleError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": pages})
}

func (handler *Handler) handleError(ctx *gin.Context, err error) {
	handler.logger.Warn("request failed", zap.Error(err))

	switch {
	case errors.Is(err, errs.ErrInvalidInput):
		ctx.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, errs.ErrForbidden):
		ctx.JSON(403, gin.H{"error": "forbidden"})
	case errors.Is(err, errs.ErrConflict):
		ctx.JSON(409, gin.H{"error": err.Error()})
	case errors.Is(err, errs.ErrNotFound):
		ctx.JSON(404, gin.H{"error": err.Error()})
	default:
		ctx.JSON(500, gin.H{"error": "internal server error"})
	}
}
