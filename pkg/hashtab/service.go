package hashtab

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rmitchellscott/rm-qmd-hasher/internal/logging"
)

type VersionInfo struct {
	Version     string   `json:"version"`
	Devices     []string `json:"devices"`
	DeviceCount int      `json:"deviceCount"`
}

type Service struct {
	hashtables      []*Hashtab
	dir             string
	mu              sync.RWMutex
	modTimes        map[string]time.Time
	pathByName      map[string]string
	byVersion       map[string][]*Hashtab
	lastReloadCheck time.Time
}

func NewService(dir string) (*Service, error) {
	service := &Service{
		hashtables: make([]*Hashtab, 0),
		dir:        dir,
		modTimes:   make(map[string]time.Time),
		pathByName: make(map[string]string),
		byVersion:  make(map[string][]*Hashtab),
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logging.Warn(logging.ComponentHashtab, "Hashtable directory does not exist: %s", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create hashtable directory: %w", err)
		}
		logging.Info(logging.ComponentHashtab, "Created hashtable directory: %s", dir)
		return service, nil
	}

	err := service.loadHashtables()
	if err != nil {
		return nil, err
	}

	return service, nil
}

func (s *Service) loadHashtables() error {
	loadedNames := make(map[string]string)

	err := filepath.WalkDir(s.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		filename := filepath.Base(path)

		if existingPath, exists := loadedNames[filename]; exists {
			logging.Warn(logging.ComponentHashtab, "Skipping duplicate hashtable file %s (already loaded from %s)", path, existingPath)
			return nil
		}

		logging.Info(logging.ComponentHashtab, "Loading hashtable: %s", filename)

		ht, err := Load(path)
		if err != nil {
			logging.Error(logging.ComponentHashtab, "Failed to load hashtable %s: %v", filename, err)
			return nil
		}

		formatType := "hashtab (with strings)"
		if ht.IsHashlist() {
			formatType = "hashlist (hash-only)"
		}
		logging.Info(logging.ComponentHashtab, "Loaded %s: %s, %d entries, version %s, device %s", filename, formatType, len(ht.Entries), ht.OSVersion, ht.Device)

		s.hashtables = append(s.hashtables, ht)
		loadedNames[filename] = path
		s.pathByName[filename] = path

		s.byVersion[ht.OSVersion] = append(s.byVersion[ht.OSVersion], ht)

		fileInfo, err := d.Info()
		if err == nil {
			s.modTimes[path] = fileInfo.ModTime()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk hashtable directory: %w", err)
	}

	return nil
}

func (s *Service) CheckAndReload() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.lastReloadCheck) < 5*time.Second {
		return false, nil
	}
	s.lastReloadCheck = time.Now()

	currentFiles := make(map[string]time.Time)
	needsReload := false

	err := filepath.WalkDir(s.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		modTime := fileInfo.ModTime()
		currentFiles[path] = modTime

		if lastMod, exists := s.modTimes[path]; !exists || !lastMod.Equal(modTime) {
			needsReload = true
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to walk hashtable directory: %w", err)
	}

	if !needsReload {
		for path := range s.modTimes {
			if _, exists := currentFiles[path]; !exists {
				needsReload = true
				break
			}
		}
	}

	if !needsReload {
		return false, nil
	}

	logging.Info(logging.ComponentHashtab, "Detected hashtable changes, reloading...")

	s.hashtables = make([]*Hashtab, 0)
	s.modTimes = make(map[string]time.Time)
	s.pathByName = make(map[string]string)
	s.byVersion = make(map[string][]*Hashtab)

	loadedNames := make(map[string]string)

	err = filepath.WalkDir(s.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		filename := filepath.Base(path)

		if existingPath, exists := loadedNames[filename]; exists {
			logging.Warn(logging.ComponentHashtab, "Skipping duplicate hashtable file %s (already loaded from %s)", path, existingPath)
			return nil
		}

		logging.Info(logging.ComponentHashtab, "Loading hashtable: %s", filename)

		ht, err := Load(path)
		if err != nil {
			logging.Error(logging.ComponentHashtab, "Failed to load hashtable %s: %v", filename, err)
			return nil
		}

		formatType := "hashtab (with strings)"
		if ht.IsHashlist() {
			formatType = "hashlist (hash-only)"
		}
		logging.Info(logging.ComponentHashtab, "Loaded %s: %s, %d entries, version %s", filename, formatType, len(ht.Entries), ht.OSVersion)

		s.hashtables = append(s.hashtables, ht)
		loadedNames[filename] = path
		s.pathByName[filename] = path

		s.byVersion[ht.OSVersion] = append(s.byVersion[ht.OSVersion], ht)

		fileInfo, err := d.Info()
		if err == nil {
			s.modTimes[path] = fileInfo.ModTime()
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to reload hashtables: %w", err)
	}

	logging.Info(logging.ComponentHashtab, "Reload complete: %d hashtables loaded", len(s.hashtables))

	return true, nil
}

func (s *Service) GetHashtables() []*Hashtab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hashtables
}

func (s *Service) GetHashtable(name string) *Hashtab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ht := range s.hashtables {
		if ht.Name == name {
			return ht
		}
	}
	return nil
}

func (s *Service) GetHashtabsForVersion(version string) []*Hashtab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byVersion[version]
}

func (s *Service) GetVersions() []VersionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := make([]VersionInfo, 0, len(s.byVersion))
	for ver, hts := range s.byVersion {
		devices := make([]string, 0, len(hts))
		for _, ht := range hts {
			devices = append(devices, ht.Device)
		}
		sort.Strings(devices)
		versions = append(versions, VersionInfo{
			Version:     ver,
			Devices:     devices,
			DeviceCount: len(devices),
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	return versions
}

func (s *Service) GetModTimes() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]time.Time, len(s.modTimes))
	for k, v := range s.modTimes {
		result[k] = v
	}
	return result
}
