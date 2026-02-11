package serve

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Workspace & files
	mux.HandleFunc("GET /api/v1/workspace", s.handleGetWorkspace)
	mux.HandleFunc("GET /api/v1/files", s.handleListFiles)
	mux.HandleFunc("POST /api/v1/files", s.handleCreateFile)
	mux.HandleFunc("GET /api/v1/files/raw/{path...}", s.handleGetFileRaw)
	mux.HandleFunc("PUT /api/v1/files/{path...}", s.handleSaveFile)
	mux.HandleFunc("DELETE /api/v1/files/{path...}", s.handleDeleteFile)
	mux.HandleFunc("GET /api/v1/files/{path...}", s.handleGetFile)

	// Execution
	mux.HandleFunc("POST /api/v1/execute", s.handleExecuteRequest)
	mux.HandleFunc("POST /api/v1/run", s.handleRunFile)

	// Environments
	mux.HandleFunc("GET /api/v1/environments", s.handleListEnvironments)
	mux.HandleFunc("PUT /api/v1/environments/active", s.handleSelectEnvironment)
	mux.HandleFunc("GET /api/v1/environments/{name}", s.handleGetEnvironment)
	mux.HandleFunc("PUT /api/v1/environments/{name}", s.handlePutEnvironment)

	// Config
	mux.HandleFunc("GET /api/v1/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/v1/config", s.handlePutConfig)

	// History (legacy in-memory)
	mux.HandleFunc("GET /api/v1/history", s.handleGetHistory)
	mux.HandleFunc("DELETE /api/v1/history", s.handleClearHistory)

	// History (persistent, SQLite-backed)
	mux.HandleFunc("GET /api/v1/history/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/v1/history/runs/{id}", s.handleGetRun)
	mux.HandleFunc("DELETE /api/v1/history/runs", s.handleClearAllRuns)
	mux.HandleFunc("DELETE /api/v1/history/runs/{id}", s.handleDeleteRun)

	// Stress testing
	mux.HandleFunc("POST /api/v1/stress/start", s.handleStressStart)
	mux.HandleFunc("POST /api/v1/stress/stop", s.handleStressStop)
	mux.HandleFunc("GET /api/v1/stress/status", s.handleStressStatus)
	mux.HandleFunc("GET /api/v1/stress/result", s.handleStressResult)

	// Mock server
	mux.HandleFunc("POST /api/v1/mock/start", s.handleMockStart)
	mux.HandleFunc("POST /api/v1/mock/stop", s.handleMockStop)
	mux.HandleFunc("GET /api/v1/mock/routes", s.handleMockRoutes)

	// Contract testing
	mux.HandleFunc("POST /api/v1/contract/verify", s.handleContractVerify)
	mux.HandleFunc("GET /api/v1/contract/files", s.handleContractFiles)

	// Recording proxy
	mux.HandleFunc("POST /api/v1/record/start", s.handleRecordStart)
	mux.HandleFunc("POST /api/v1/record/stop", s.handleRecordStop)
	mux.HandleFunc("GET /api/v1/record/status", s.handleRecordStatus)
	mux.HandleFunc("POST /api/v1/record/export", s.handleRecordExport)
	mux.HandleFunc("DELETE /api/v1/record/clear", s.handleRecordClear)

	// Stress profiles
	mux.HandleFunc("GET /api/v1/stress/profiles", s.handleStressProfiles)
	mux.HandleFunc("POST /api/v1/stress/profiles", s.handleCreateStressProfile)
	mux.HandleFunc("PUT /api/v1/stress/profiles/{name}", s.handleUpdateStressProfile)
	mux.HandleFunc("DELETE /api/v1/stress/profiles/{name}", s.handleDeleteStressProfile)

	// Import/export
	mux.HandleFunc("POST /api/v1/import/curl", s.handleImportCurl)
	mux.HandleFunc("POST /api/v1/import/insomnia", s.handleImportInsomnia)
	mux.HandleFunc("POST /api/v1/import/openapi", s.handleImportOpenAPI)
	mux.HandleFunc("POST /api/v1/export/curl", s.handleExportCurl)

	// System
	mux.HandleFunc("GET /api/v1/system/info", s.handleSystemInfo)

	// WebSocket
	mux.HandleFunc("GET /api/v1/ws", s.handleWebSocket)

	// SPA fallback (must be last)
	if !s.config.APIOnly {
		mux.Handle("/", spaHandler())
	}
}
