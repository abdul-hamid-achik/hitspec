package serve

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/contract"
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.ProviderURL == "" {
		writeError(w, http.StatusBadRequest, "providerUrl is required")
		return
	}

	opts := []contract.Option{
		contract.WithProviderURL(req.ProviderURL),
		contract.WithVerbose(s.config.Verbose),
	}
	if req.StateHandler != "" {
		opts = append(opts, contract.WithStateHandler(req.StateHandler))
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

		result, err := verifier.VerifyFile(absPath)
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
