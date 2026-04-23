package main

import (
	"net/http"
	"op-bot/internal/app"
	"op-bot/internal/logging"
	"time"
)

// @title			Op-Bot API
// @version		1.0.0
// @description	Backend API for portfolio deployment with GitHub OAuth and resume validation. Runs as a microservice separate from the frontend.
// @BasePath		/
func main() {
	// Load configuration into AppContext
	ctx := LoadAppContext()
	defer func() {
		if ctx.Logger != nil {
			ctx.Logger.Info("server.shutdown")
		}
	}()

	// Setup logger
	ctx.Logger = logging.SetupLogger(ctx.LogLevel)

	handler := app.NewHTTPHandler(app.Dependencies{
		AuthGitHubStart:    handleAuthGitHubStart,
		AuthGitHubCallback: handleAuthGitHubCallback,
		APIGitHubMe:        handleAPIGitHubMe,
		APIGitHubRepos:     handleAPIGitHubRepos,
		APIGitHubLogout:    handleAPIGitHubLogout,
		APIResumeValidate:  handleAPIResumeValidate,
		APIResumeParsePDF:  handleAPIResumeParsePDF,
		APIGitHubDeploy:    handleAPIGitHubDeploy,
		SwaggerUI:          handleSwaggerUI,
		OpenAPISpec:        handleOpenAPISpec,
	}, buildHandler)
	server := &http.Server{
		Addr:              ctx.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx.Logger.Info("server.starting", "addr", ctx.HTTPAddr, "port", ctx.Port)
	ctx.Logger.Info("server.swagger_ui", "url", "http://localhost:"+ctx.Port+"/swagger")
	if err := server.ListenAndServe(); err != nil {
		ctx.Logger.Error("server.listen_failed", "error", err)
		return
	}
}
