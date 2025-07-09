package aptosstream

type VersionTracker interface {
	GetVersion() (uint64, error)
	SetVersion(version uint64) error
}

type InMemoryVersionTracker struct {
	version uint64
}

func (v *InMemoryVersionTracker) GetVersion() (uint64, error) {
	return v.version, nil
}

func (v *InMemoryVersionTracker) SetVersion(version uint64) error {
	if version > v.version {
		v.version = version
	}
	return nil
}
