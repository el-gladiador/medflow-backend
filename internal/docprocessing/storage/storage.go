package storage

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
)

// TempStorage provides in-memory storage for extraction jobs.
// Images are processed in RAM only and cryptographically zeroed after use.
// Jobs are automatically cleaned up after a TTL.
type TempStorage struct {
	mu   sync.RWMutex
	jobs map[string]*domain.ExtractionJob
	ttl  time.Duration
}

// NewTempStorage creates a new in-memory temp storage with the given TTL
func NewTempStorage(ttl time.Duration) *TempStorage {
	s := &TempStorage{
		jobs: make(map[string]*domain.ExtractionJob),
		ttl:  ttl,
	}
	go s.cleanupLoop()
	return s
}

// GenerateJobID creates a cryptographically random job ID
func GenerateJobID() string {
	b := make([]byte, 16)
	rand.Read(b)
	const hex = "0123456789abcdef"
	id := make([]byte, 32)
	for i, v := range b {
		id[i*2] = hex[v>>4]
		id[i*2+1] = hex[v&0x0f]
	}
	return string(id)
}

// StoreJob stores an extraction job
func (s *TempStorage) StoreJob(job *domain.ExtractionJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = job
}

// GetJob retrieves an extraction job by ID
func (s *TempStorage) GetJob(jobID string) *domain.ExtractionJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[jobID]
}

// UpdateJob updates an existing extraction job
func (s *TempStorage) UpdateJob(jobID string, update func(*domain.ExtractionJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		update(job)
	}
}

// DeleteJob removes a job from storage
func (s *TempStorage) DeleteJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, jobID)
}

// ZeroBytes overwrites a byte slice with zeros for secure deletion.
// This prevents sensitive image data from lingering in memory.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// cleanupLoop periodically removes expired jobs
func (s *TempStorage) cleanupLoop() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanup()
	}
}

func (s *TempStorage) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-s.ttl)
	for id, job := range s.jobs {
		if job.CreatedAt.Before(cutoff) {
			delete(s.jobs, id)
		}
	}
}
