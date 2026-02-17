package auth

import (
	"strings"
	"sync"
	"time"
)

// AuditEntry represents a single authorization decision event.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	UserEmail string    `json:"user_email"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Path      string    `json:"path"`
	Method    string    `json:"method"`
	OrgID     string    `json:"org_id,omitempty"`
	Allowed   bool      `json:"allowed"`
	Reason    string    `json:"reason,omitempty"`
	IP        string    `json:"ip"`
}

// AuditLog is a thread-safe in-memory ring buffer of authorization events.
type AuditLog struct {
	mu      sync.RWMutex
	entries []AuditEntry
	size    int
	head    int
	count   int
}

// NewAuditLog creates an audit log with the given maximum capacity.
func NewAuditLog(maxEntries int) *AuditLog {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &AuditLog{
		entries: make([]AuditEntry, maxEntries),
		size:    maxEntries,
	}
}

// Record adds an entry to the ring buffer.
func (al *AuditLog) Record(entry AuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.entries[al.head] = entry
	al.head = (al.head + 1) % al.size
	al.count++
}

// List returns the most recent n entries, newest first.
func (al *AuditLog) List(n int) []AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	available := al.count
	if available > al.size {
		available = al.size
	}
	if n <= 0 || n > available {
		n = available
	}

	result := make([]AuditEntry, 0, n)
	idx := (al.head - 1 + al.size) % al.size
	for i := 0; i < n; i++ {
		result = append(result, al.entries[idx])
		idx = (idx - 1 + al.size) % al.size
	}
	return result
}

// ListFiltered returns entries matching the given filter criteria, newest first.
func (al *AuditLog) ListFiltered(userEmail, resource, action string, limit int) []AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	available := al.count
	if available > al.size {
		available = al.size
	}
	if limit <= 0 {
		limit = 100
	}

	result := make([]AuditEntry, 0, limit)
	idx := (al.head - 1 + al.size) % al.size
	for i := 0; i < available && len(result) < limit; i++ {
		entry := al.entries[idx]
		if matchesFilter(entry, userEmail, resource, action) {
			result = append(result, entry)
		}
		idx = (idx - 1 + al.size) % al.size
	}
	return result
}

// Count returns the total number of entries recorded (may exceed capacity).
func (al *AuditLog) Count() int {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.count
}

func matchesFilter(entry AuditEntry, userEmail, resource, action string) bool {
	if userEmail != "" && !strings.EqualFold(entry.UserEmail, userEmail) {
		return false
	}
	if resource != "" && entry.Resource != resource {
		return false
	}
	if action != "" && entry.Action != action {
		return false
	}
	return true
}
