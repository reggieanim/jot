package httputil

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(corsOrigins string) *gin.Engine {
	allowed := make(map[string]bool)
	for _, o := range strings.Split(corsOrigins, ",") {
		o = strings.ToLower(strings.TrimSpace(o))
		if o != "" {
			allowed[o] = true
		}
	}

	router := gin.New()
	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowOriginFunc: func(origin string) bool {
			return allowed[strings.ToLower(strings.TrimSpace(origin))]
		},
		MaxAge: 12 * time.Hour,
	}))
	router.Use(gin.Recovery())
	router.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"status": "ok"})
	})
	return router
}
