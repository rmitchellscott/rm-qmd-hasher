package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rmitchellscott/rm-qmd-hasher/internal/jobs"
	"github.com/rmitchellscott/rm-qmd-hasher/internal/logging"
	"github.com/rmitchellscott/rm-qmd-hasher/internal/qmldiff"
	"github.com/rmitchellscott/rm-qmd-hasher/pkg/gcdcache"
)

type APIHandler struct {
	qmldiffService *qmldiff.Service
	gcdCache       *gcdcache.Service
	jobStore       *jobs.Store
}

func NewAPIHandler(qmldiffService *qmldiff.Service, gcdCache *gcdcache.Service, jobStore *jobs.Store) *APIHandler {
	return &APIHandler{
		qmldiffService: qmldiffService,
		gcdCache:       gcdCache,
		jobStore:       jobStore,
	}
}

func (h *APIHandler) Hash(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		logging.Error(logging.ComponentHandler, "Failed to parse multipart form: %v", err)
		writeJSONError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	version := r.FormValue("version")
	if version == "" {
		writeJSONError(w, http.StatusBadRequest, "version is required")
		return
	}

	var fileHeaders []*multipart.FileHeader
	var filePaths []string

	if files := r.MultipartForm.File["files"]; len(files) > 0 {
		fileHeaders = files
		filePaths = r.MultipartForm.Value["paths"]
	} else {
		file, header, err := r.FormFile("file")
		if err != nil {
			logging.Error(logging.ComponentHandler, "No files uploaded: %v", err)
			writeJSONError(w, http.StatusBadRequest, "No file uploaded or invalid form data")
			return
		}
		file.Close()
		fileHeaders = []*multipart.FileHeader{header}
		filePaths = []string{header.Filename}
	}

	inputDir, err := os.MkdirTemp("", "hash-input-*")
	if err != nil {
		logging.Error(logging.ComponentHandler, "Failed to create input temp directory: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to create temp directory")
		return
	}

	outputDir, err := os.MkdirTemp("", "hash-output-*")
	if err != nil {
		os.RemoveAll(inputDir)
		logging.Error(logging.ComponentHandler, "Failed to create output temp directory: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to create temp directory")
		return
	}

	qmdFiles := make([]string, 0, len(fileHeaders))
	relPaths := make([]string, 0, len(fileHeaders))

	for i, fileHeader := range fileHeaders {
		file, err := fileHeader.Open()
		if err != nil {
			logging.Error(logging.ComponentHandler, "Failed to open uploaded file %s: %v", fileHeader.Filename, err)
			os.RemoveAll(inputDir)
			os.RemoveAll(outputDir)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open file %s", fileHeader.Filename))
			return
		}

		var relativePath string
		if i < len(filePaths) && filePaths[i] != "" {
			relativePath = filepath.Clean(filePaths[i])
		} else {
			relativePath = filepath.Clean(fileHeader.Filename)
		}

		if !strings.HasSuffix(strings.ToLower(relativePath), ".qmd") {
			file.Close()
			continue
		}

		inputPath := filepath.Join(inputDir, relativePath)
		outputPath := filepath.Join(outputDir, relativePath)

		cleanInputDir := filepath.Clean(inputDir) + string(os.PathSeparator)
		cleanInputPath := filepath.Clean(inputPath)
		if !strings.HasPrefix(cleanInputPath+string(os.PathSeparator), cleanInputDir) {
			file.Close()
			os.RemoveAll(inputDir)
			os.RemoveAll(outputDir)
			logging.Warn(logging.ComponentHandler, "Path traversal attempt detected: %s", relativePath)
			writeJSONError(w, http.StatusBadRequest, "Invalid file path")
			return
		}

		if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
			file.Close()
			os.RemoveAll(inputDir)
			os.RemoveAll(outputDir)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create directory for file %s", fileHeader.Filename))
			return
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			file.Close()
			os.RemoveAll(inputDir)
			os.RemoveAll(outputDir)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create output directory for file %s", fileHeader.Filename))
			return
		}

		inputFile, err := os.Create(inputPath)
		if err != nil {
			file.Close()
			os.RemoveAll(inputDir)
			os.RemoveAll(outputDir)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file %s", fileHeader.Filename))
			return
		}

		bytesWritten, err := io.Copy(inputFile, file)
		file.Close()
		inputFile.Close()

		if err != nil {
			os.RemoveAll(inputDir)
			os.RemoveAll(outputDir)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file %s", fileHeader.Filename))
			return
		}

		if bytesWritten == 0 {
			logging.Warn(logging.ComponentHandler, "Skipping empty file: %s", fileHeader.Filename)
			continue
		}

		qmdFiles = append(qmdFiles, inputPath)
		relPaths = append(relPaths, relativePath)
	}

	if len(qmdFiles) == 0 {
		os.RemoveAll(inputDir)
		os.RemoveAll(outputDir)
		writeJSONError(w, http.StatusBadRequest, "No .qmd files uploaded")
		return
	}

	logging.Info(logging.ComponentHandler, "Received %d QMD file(s) for hashing with version %s", len(qmdFiles), version)

	jobID := uuid.New().String()
	job := h.jobStore.Create(jobID)
	job.FileCount = len(qmdFiles)
	h.jobStore.SetOutputDir(jobID, outputDir)

	go h.processHashJob(jobID, version, qmdFiles, relPaths, inputDir, outputDir)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"jobId": jobID,
	})
}

func (h *APIHandler) processHashJob(jobID, version string, qmdFiles, relPaths []string, inputDir, outputDir string) {
	defer os.RemoveAll(inputDir)

	h.jobStore.UpdateWithOperation(jobID, "running", "Getting GCD hashtab", nil, "preparing")

	gcdPath, err := h.gcdCache.GetGCDHashtab(version)
	if err != nil {
		logging.Error(logging.ComponentHandler, "Failed to get GCD hashtab for version %s: %v", version, err)
		h.jobStore.Update(jobID, "error", fmt.Sprintf("Version %s not available: %v", version, err), nil)
		os.RemoveAll(outputDir)
		return
	}

	h.jobStore.UpdateWithOperation(jobID, "running", "Hashing files", nil, "hashing")

	results := make([]jobs.FileResult, 0, len(qmdFiles))
	successCount := 0

	for i, inputPath := range qmdFiles {
		relPath := relPaths[i]
		outputPath := filepath.Join(outputDir, relPath)

		if err := copyFile(inputPath, outputPath); err != nil {
			logging.Error(logging.ComponentHandler, "Failed to copy file %s: %v", relPath, err)
			results = append(results, jobs.FileResult{
				Name:   relPath,
				Path:   relPath,
				Status: "error",
				Error:  fmt.Sprintf("Failed to copy file: %v", err),
			})
			continue
		}

		if err := h.qmldiffService.HashDiffs(gcdPath, outputPath); err != nil {
			logging.Error(logging.ComponentHandler, "Failed to hash file %s: %v", relPath, err)
			results = append(results, jobs.FileResult{
				Name:   relPath,
				Path:   relPath,
				Status: "error",
				Error:  fmt.Sprintf("Hashing failed: %v", err),
			})
			os.Remove(outputPath)
			continue
		}

		results = append(results, jobs.FileResult{
			Name:   relPath,
			Path:   relPath,
			Status: "success",
		})
		successCount++

		progress := int(float64(i+1) / float64(len(qmdFiles)) * 100)
		h.jobStore.UpdateProgress(jobID, progress)
	}

	h.jobStore.SetFiles(jobID, results)

	if successCount == 0 {
		h.jobStore.Update(jobID, "error", "All files failed to hash", nil)
		os.RemoveAll(outputDir)
		return
	}

	logging.Info(logging.ComponentHandler, "Hashing complete for job %s: %d/%d files successful", jobID, successCount, len(qmdFiles))
	h.jobStore.Update(jobID, "success", fmt.Sprintf("Hashed %d file(s)", successCount), nil)
}

func (h *APIHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	versions := h.gcdCache.GetVersions()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": versions,
		"count":    len(versions),
	})
}

func (h *APIHandler) GetResults(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	job, ok := h.jobStore.Get(jobID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if job.Status != "success" && job.Status != "error" {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    job.Status,
			"message":   job.Message,
			"progress":  job.Progress,
			"operation": job.Operation,
			"fileCount": job.FileCount,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    job.Status,
		"message":   job.Message,
		"files":     job.Files,
		"fileCount": job.FileCount,
	})
}

func (h *APIHandler) Download(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	job, ok := h.jobStore.Get(jobID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}

	if job.Status != "success" {
		writeJSONError(w, http.StatusBadRequest, "Job not complete or failed")
		return
	}

	if job.OutputDir == "" {
		writeJSONError(w, http.StatusInternalServerError, "Output directory not available")
		return
	}

	successFiles := make([]jobs.FileResult, 0)
	for _, f := range job.Files {
		if f.Status == "success" {
			successFiles = append(successFiles, f)
		}
	}

	if len(successFiles) == 0 {
		writeJSONError(w, http.StatusBadRequest, "No successfully hashed files to download")
		return
	}

	if len(successFiles) == 1 {
		filePath := filepath.Join(job.OutputDir, successFiles[0].Path)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(successFiles[0].Name)))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filePath)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\"hashed-files.zip\"")
	w.Header().Set("Content-Type", "application/zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, f := range successFiles {
		filePath := filepath.Join(job.OutputDir, f.Path)

		file, err := os.Open(filePath)
		if err != nil {
			logging.Error(logging.ComponentHandler, "Failed to open file for zip: %s: %v", filePath, err)
			continue
		}

		zipEntry, err := zipWriter.Create(f.Path)
		if err != nil {
			file.Close()
			logging.Error(logging.ComponentHandler, "Failed to create zip entry: %s: %v", f.Path, err)
			continue
		}

		_, err = io.Copy(zipEntry, file)
		file.Close()
		if err != nil {
			logging.Error(logging.ComponentHandler, "Failed to write to zip: %s: %v", f.Path, err)
		}
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
