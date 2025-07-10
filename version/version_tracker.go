package version

import "sync"

type VersionTracker interface {
	GetVersion() (uint64, error)
	SetVersion(version uint64) error
}

type InMemoryVersionTracker struct {
	mu      sync.Mutex
	version uint64
}

func (v *InMemoryVersionTracker) GetVersion() (uint64, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.version, nil
}

// SetVersion updates the tracker's version, but only if the new version is greater
// than the currently stored version.
func (v *InMemoryVersionTracker) SetVersion(version uint64) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if version > v.version {
		v.version = version
	}
	return nil
}
