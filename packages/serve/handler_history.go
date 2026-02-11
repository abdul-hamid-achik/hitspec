package serve

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/history"
)

// --- Legacy in-memory history (unchanged) ---

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.history.Entries())
}

func (s *Server) handleClearHistory(w http.ResponseWriter, r *http.Request) {
	s.history.Clear()
	w.WriteHeader(http.StatusNoContent)
}

// --- Persistent history endpoints ---

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeError(w, http.StatusServiceUnavailable, "history database not available")
		return
	}

	limit := int64(20)
	offset := int64(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			offset = n
		}
	}

	ctx := r.Context()
	runs, err := s.historyStore.Queries().ListRuns(ctx, history.ListRunsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := s.historyStore.Queries().CountRuns(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]HistoryRunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, convertRunToDTO(run))
	}

	writeJSON(w, http.StatusOK, HistoryListDTO{
		Runs:   dtos,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeError(w, http.StatusServiceUnavailable, "history database not available")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	ctx := r.Context()
	run, err := s.historyStore.Queries().GetRun(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	dto := convertRunToDTO(run)

	// Load results for this run
	results, err := s.historyStore.Queries().ListResultsByRun(ctx, id)
	if err == nil {
		dto.Results = make([]HistoryResultDTO, 0, len(results))
		for _, res := range results {
			rdto := convertResultToDTO(res)

			// Load assertions for each result
			assertions, err := s.historyStore.Queries().ListAssertionsByResult(ctx, res.ID)
			if err == nil && len(assertions) > 0 {
				rdto.Assertions = make([]HistoryAssertionDTO, 0, len(assertions))
				for _, a := range assertions {
					rdto.Assertions = append(rdto.Assertions, convertAssertionToDTO(a))
				}
			}

			dto.Results = append(dto.Results, rdto)
		}
	}

	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeError(w, http.StatusServiceUnavailable, "history database not available")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := s.historyStore.Queries().DeleteRun(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClearAllRuns(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeError(w, http.StatusServiceUnavailable, "history database not available")
		return
	}

	if err := s.historyStore.Queries().ClearAllRuns(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// recordRunToHistory persists a run result to the history store in a goroutine.
// It is safe to call even if historyStore is nil.
func (s *Server) recordRunToHistory(filePath, environment string, result *RunResultDTO) {
	if s.historyStore == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		runID, err := s.historyStore.RecordRun(ctx, filePath, environment)
		if err != nil {
			return
		}

		var passed, failed, skipped int64
		for _, rr := range result.Results {
			statusCode := 0
			if rr.Response != nil {
				statusCode = rr.Response.StatusCode
			}
			method, url := "", ""
			if rr.Request != nil {
				method = rr.Request.Method
				url = rr.Request.URL
			}

			bodyPreview := ""
			if rr.Response != nil && len(rr.Response.Body) > 0 {
				preview := rr.Response.Body
				if len(preview) > 65536 {
					preview = preview[:65536]
				}
				bodyPreview = preview
			}

			resultID, err := s.historyStore.RecordResult(ctx, runID,
				rr.Name, method, url, statusCode,
				int64(rr.Duration),
				rr.Passed, rr.Skipped,
				rr.Error, rr.Description, bodyPreview,
			)
			if err != nil {
				continue
			}

			if len(rr.Assertions) > 0 {
				records := make([]history.AssertionRecord, 0, len(rr.Assertions))
				for _, a := range rr.Assertions {
					expected := ""
					if a.Expected != nil {
						expected = formatAny(a.Expected)
					}
					actual := ""
					if a.Actual != nil {
						actual = formatAny(a.Actual)
					}
					records = append(records, history.AssertionRecord{
						Operator: a.Operator,
						Subject:  a.Subject,
						Expected: expected,
						Actual:   actual,
						Passed:   a.Passed,
						Message:  a.Message,
					})
				}
				_ = s.historyStore.RecordAssertions(ctx, resultID, records)
			}

			if rr.Passed {
				passed++
			} else if rr.Skipped {
				skipped++
			} else {
				failed++
			}
		}

		dur := time.Duration(result.Duration) * time.Millisecond
		_ = s.historyStore.FinishRun(ctx, runID, dur, passed, failed, skipped, passed+failed+skipped)
	}()
}

// --- DTO converters ---

func convertRunToDTO(run history.Run) HistoryRunDTO {
	dto := HistoryRunDTO{
		ID:         run.ID,
		FilePath:   run.FilePath,
		StartedAt:  run.StartedAt.UTC().Format(time.RFC3339),
		DurationMs: run.DurationMs,
		Passed:     run.Passed,
		Failed:     run.Failed,
		Skipped:    run.Skipped,
		Total:      run.Total,
	}
	if run.Environment.Valid {
		dto.Environment = run.Environment.String
	}
	if run.FinishedAt.Valid {
		dto.FinishedAt = run.FinishedAt.Time.UTC().Format(time.RFC3339)
	}
	return dto
}

func convertResultToDTO(res history.Result) HistoryResultDTO {
	dto := HistoryResultDTO{
		ID:          res.ID,
		RequestName: res.RequestName,
		Method:      res.Method,
		URL:         res.Url,
		DurationMs:  res.DurationMs,
		Passed:      res.Passed,
		Skipped:     res.Skipped,
	}
	if res.StatusCode.Valid {
		dto.StatusCode = int(res.StatusCode.Int64)
	}
	if res.Error.Valid {
		dto.Error = res.Error.String
	}
	if res.Description.Valid {
		dto.Description = res.Description.String
	}
	if res.BodyPreview.Valid {
		dto.BodyPreview = res.BodyPreview.String
	}
	return dto
}

func convertAssertionToDTO(a history.Assertion) HistoryAssertionDTO {
	dto := HistoryAssertionDTO{
		ID:       a.ID,
		Operator: a.Operator,
		Subject:  a.Subject,
		Passed:   a.Passed,
	}
	if a.Expected.Valid {
		dto.Expected = a.Expected.String
	}
	if a.Actual.Valid {
		dto.Actual = a.Actual.String
	}
	if a.Message.Valid {
		dto.Message = a.Message.String
	}
	return dto
}

func formatAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
