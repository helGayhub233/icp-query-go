package server

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/imxw/icp-query-go/internal/config"
	"github.com/imxw/icp-query-go/internal/store"
	"github.com/imxw/icp-query-go/internal/task"
	"github.com/imxw/icp-query-go/server/handler"
	"github.com/imxw/icp-query-go/server/middleware"
)

// Server holds all dependencies for the HTTP server.
type Server struct {
	cfg   *config.Config
	beian *beian.Beian
	db    *store.Store
	tm    *task.Manager
	webUI bool
}

// New creates a new Server with the given dependencies.
func New(cfg *config.Config, b *beian.Beian, db *store.Store, webUI bool) *Server {
	tm := task.NewManager(b, db)
	return &Server{
		cfg:   cfg,
		beian: b,
		db:    db,
		tm:    tm,
		webUI: webUI,
	}
}

// SetupRouter creates and configures the gin engine.
func (s *Server) SetupRouter(webFS embed.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// Static files
	sub, err := fs.Sub(webFS, "web/static")
	if err == nil {
		r.StaticFS("/static", http.FS(sub))
	}

	// Templates
	tmpl := template.Must(template.New("").ParseFS(webFS, "web/templates/*"))
	r.SetHTMLTemplate(tmpl)

	// API routes via Handler struct injection
	h := handler.New(s.beian, s.db, s.cfg, s.tm)
	h.RegisterRoutes(r)

	// Web UI
	handler.RegisterUIRoutes(r, s.webUI)

	return r
}

// Run starts the HTTP server.
func (s *Server) Run(webFS embed.FS) error {
	r := s.SetupRouter(webFS)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	slog.Info("server listening", "addr", addr)
	return r.Run(addr)
}
