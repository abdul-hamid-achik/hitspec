package serve

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/contract"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// ContractVerifyReq is the request body for POST /contract/verify.
type ContractVerifyReq struct {
	Files        []string `json:"files"`
	ProviderURL  string   `json:"providerUrl"`
	StateHandler string   `json:"stateHandler,omitempty"`
}

// ContractResultDTO is the result of contract verification.
type ContractResultDTO struct {
	File     string                   `json:"file"`
	Passed   int                      `json:"passed"`
	Failed   int                      `json:"failed"`
	Skipped  int                      `json:"skipped"`
	Duration float64                  `json:"duration"`
	Results  []ContractInteractionDTO `json:"results"`
}

// ContractInteractionDTO is a single contract interaction result.
type ContractInteractionDTO struct {
	Name     string  `json:"name"`
	Provider string  `json:"provider,omitempty"`
	State    string  `json:"state,omitempty"`
	Passed   bool    `json:"passed"`
	Error    string  `json:"error,omitempty"`
	Duration float64 `json:"duration"`
}

// ContractStatusDTO is the response for GET /contract/status.
type ContractStatusDTO struct {
	Files   []string            `json:"files"`
	Results []ContractResultDTO `json:"results,omitempty"`
}

func (s *Server) handleContractVerify(w http.ResponseWriter, r *http.Request) {
	var req ContractVerifyReq
	if err := readJSON(r, &req); err != nil {
		writeReadError(w, err)
		return
	}

	// A state handler is an executable the server would run. A REST caller must
	// never be able to pick one by default, so it needs the same explicit
	// --allow-shell opt-in as shell execution elsewhere, and even then it has to
	// live inside the workspace.
	stateHandler := ""
	if req.StateHandler != "" {
		if !s.config.AllowShell {
			writeError(w, http.StatusForbidden, "stateHandler runs an executable and requires the server to be started with --allow-shell")
			return
		}
		resolved, err := s.resolveStateHandler(req.StateHandler)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		stateHandler = resolved
	}

	if req.ProviderURL == "" {
		writeError(w, http.StatusBadRequest, "providerUrl is required")
		return
	}

	_, clientOpts := s.runSettings("")
	opts := []contract.Option{
		contract.WithHTTPClient(hitspechttp.NewClient(clientOpts...)),
		contract.WithProviderURL(req.ProviderURL),
		contract.WithVerbose(s.config.Verbose),
	}
	if stateHandler != "" {
		opts = append(opts, contract.WithStateHandler(stateHandler))
	}

	verifier := contract.NewVerifier(opts...)

	var results []ContractResultDTO

	files := req.Files
	if len(files) == 0 {
		// Use all hitspec files in workspace
		collected, err := collectHitspecFiles(s.config.WorkDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan workspace: "+err.Error())
			return
		}
		files = collected
	}

	for _, file := range files {
		absPath := file
		if !filepath.IsAbs(file) {
			absPath = filepath.Join(s.config.WorkDir, file)
		}

		if !isPathWithin(s.config.WorkDir, absPath) {
			continue
		}

		result, err := verifier.VerifyFileContext(r.Context(), absPath)
		if err != nil && r.Context().Err() != nil {
			// The client went away mid-run: that is a cancellation, not a
			// parse failure, and nobody is left to read a response.
			return
		}
		if err != nil {
			results = append(results, ContractResultDTO{
				File: file,
				Results: []ContractInteractionDTO{{
					Name:   "parse",
					Passed: false,
					Error:  err.Error(),
				}},
			})
			continue
		}

		dto := ContractResultDTO{
			File:     file,
			Passed:   result.Passed,
			Failed:   result.Failed,
			Skipped:  result.Skipped,
			Duration: float64(result.Duration) / float64(time.Millisecond),
		}

		for _, ir := range result.Results {
			interaction := ContractInteractionDTO{
				Name:     ir.Name,
				Provider: ir.Provider,
				State:    ir.State,
				Passed:   ir.Passed,
				Duration: float64(ir.Duration) / float64(time.Millisecond),
			}
			if ir.Error != nil {
				interaction.Error = ir.Error.Error()
			}
			dto.Results = append(dto.Results, interaction)
		}

		results = append(results, dto)
	}

	writeJSON(w, http.StatusOK, results)
}

// resolveStateHandler resolves a requested state handler to an absolute path
// (relative paths are taken from the workspace, symlinks are followed) and
// verifies that it lies inside the workspace. The resolved path is returned so
// that exec never performs a PATH lookup on a bare name.
func (s *Server) resolveStateHandler(handler string) (string, error) {
	abs := handler
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.config.WorkDir, abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("stateHandler %q cannot be resolved inside the workspace", handler)
	}
	if !isPathWithin(s.config.WorkDir, resolved) {
		return "", fmt.Errorf("stateHandler %q is outside the workspace", handler)
	}
	return resolved, nil
}

func (s *Server) handleContractFiles(w http.ResponseWriter, r *http.Request) {
	files, err := collectHitspecFiles(s.config.WorkDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan workspace: "+err.Error())
		return
	}
	relFiles := make([]string, 0, len(files))
	for _, f := range files {
		rel, err := filepath.Rel(s.config.WorkDir, f)
		if err == nil {
			relFiles = append(relFiles, rel)
		}
	}
	writeJSON(w, http.StatusOK, ContractStatusDTO{Files: relFiles})
}
