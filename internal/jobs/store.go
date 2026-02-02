package jobs

import (
	"sync"
	"time"
)

type FileResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Job struct {
	Status      string                 `json:"status"`
	Message     string                 `json:"message"`
	Data        map[string]string      `json:"data,omitempty"`
	Progress    int                    `json:"progress"`
	Operation   string                 `json:"operation,omitempty"`
	Files       []FileResult           `json:"files,omitempty"`
	OutputDir   string                 `json:"-"`
	FileCount   int                    `json:"fileCount,omitempty"`
	CompletedAt *time.Time             `json:"-"`
}

type Store struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	watchers map[string][]chan *Job
}

func NewStore() *Store {
	s := &Store{
		jobs:     make(map[string]*Job),
		watchers: make(map[string][]chan *Job),
	}
	go s.startCleanup()
	return s
}

func (s *Store) Create(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	j := &Job{
		Status:   "pending",
		Message:  "Job created",
		Progress: 0,
		Files:    []FileResult{},
	}
	s.jobs[id] = j
	s.watchers[id] = []chan *Job{}
	return j
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *Store) Update(id, status, message string, data map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = message
		if data != nil {
			j.Data = data
		}
		if (status == "success" || status == "error") && j.CompletedAt == nil {
			now := time.Now()
			j.CompletedAt = &now
		}
		s.broadcastLocked(id)
	}
}

func (s *Store) UpdateProgress(id string, p int) {
	if p < 0 {
		p = 0
	} else if p > 100 {
		p = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Progress = p
		s.broadcastLocked(id)
	}
}

func (s *Store) UpdateWithOperation(id, status, message string, data map[string]string, operation string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = message
		if data != nil {
			j.Data = data
		}
		j.Operation = operation
		if (status == "success" || status == "error") && j.CompletedAt == nil {
			now := time.Now()
			j.CompletedAt = &now
		}
		s.broadcastLocked(id)
	}
}

func (s *Store) SetOutputDir(id string, outputDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.OutputDir = outputDir
	}
}

func (s *Store) SetFiles(id string, files []FileResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Files = files
		j.FileCount = len(files)
	}
}

func (s *Store) AddFile(id string, file FileResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Files = append(j.Files, file)
		j.FileCount = len(j.Files)
		s.broadcastLocked(id)
	}
}

func (s *Store) Subscribe(id string) (<-chan *Job, func()) {
	ch := make(chan *Job, 10)

	s.mu.Lock()
	s.watchers[id] = append(s.watchers[id], ch)
	job := s.jobs[id]
	s.mu.Unlock()

	if job != nil {
		jobCopy := s.copyJob(job)
		ch <- jobCopy
	}

	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		watchers := s.watchers[id]
		for i, c := range watchers {
			if c == ch {
				s.watchers[id] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		close(ch)
	}

	return ch, unsubscribe
}

func (s *Store) copyJob(job *Job) *Job {
	jobCopy := &Job{
		Status:    job.Status,
		Message:   job.Message,
		Data:      make(map[string]string),
		Progress:  job.Progress,
		Operation: job.Operation,
		FileCount: job.FileCount,
	}
	for k, v := range job.Data {
		jobCopy.Data[k] = v
	}
	if len(job.Files) > 0 {
		jobCopy.Files = make([]FileResult, len(job.Files))
		copy(jobCopy.Files, job.Files)
	}
	return jobCopy
}

func (s *Store) broadcastLocked(id string) {
	job := s.jobs[id]
	if job == nil {
		return
	}

	jobCopy := s.copyJob(job)

	for _, ch := range s.watchers[id] {
		select {
		case ch <- jobCopy:
		default:
		}
	}
}

func (s *Store) Cleanup(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.watchers[id] {
		close(ch)
	}

	delete(s.watchers, id)
	delete(s.jobs, id)
}

func (s *Store) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupOldJobs()
	}
}

func (s *Store) cleanupOldJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	ttl := 10 * time.Minute

	for id, job := range s.jobs {
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > ttl {
			if len(s.watchers[id]) == 0 {
				delete(s.jobs, id)
				delete(s.watchers, id)
			}
		}
	}
}
