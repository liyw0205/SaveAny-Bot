package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/config"
)

type ConfigWebServerOptions struct {
	ConfigPath string
	Host       string
	Port       int
	Token      string
}

func NewConfigWebServer(ctx context.Context, opts ConfigWebServerOptions) *http.Server {
	mux := http.NewServeMux()
	RegisterConfigEditorRoutes(ctx, mux, opts.ConfigPath, WithConfigEditorAutoOpenDatabase(true))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/config", http.StatusFound)
			return
		}
		NotFoundHandler(w, r)
	})

	var handler http.Handler = mux
	handler = TokenAuthMiddleware(func() string {
		if opts.Token != "" {
			return opts.Token
		}
		return config.C().API.Token
	})(handler)
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)

	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

func StartConfigWebServer(ctx context.Context, opts ConfigWebServerOptions) (*http.Server, error) {
	server := NewConfigWebServer(ctx, opts)
	logger := log.FromContext(ctx).With("module", "config-web")
	logger.Infof("Starting config web server on %s", server.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Config web server error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Errorf("Config web server shutdown error: %v", err)
		}
	}()
	return server, nil
}
