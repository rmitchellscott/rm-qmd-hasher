package gcdcache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rmitchellscott/rm-qmd-hasher/internal/logging"
	"github.com/rmitchellscott/rm-qmd-hasher/pkg/hashtab"
)

type GCDHashtab struct {
	Version       string
	Path          string
	SourceModTime time.Time
	DeviceCount   int
}

type Service struct {
	gcdDir         string
	qmldiffBinary  string
	hashtabService *hashtab.Service
	gcdHashtabs    map[string]*GCDHashtab
	sourceModTimes map[string]map[string]time.Time
	mu             sync.RWMutex
}

func NewService(gcdDir, qmldiffBinary string, hashtabService *hashtab.Service) (*Service, error) {
	if err := os.MkdirAll(gcdDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create GCD hashtab directory: %w", err)
	}

	service := &Service{
		gcdDir:         gcdDir,
		qmldiffBinary:  qmldiffBinary,
		hashtabService: hashtabService,
		gcdHashtabs:    make(map[string]*GCDHashtab),
		sourceModTimes: make(map[string]map[string]time.Time),
	}

	return service, nil
}

func (s *Service) GenerateAll() error {
	versions := s.hashtabService.GetVersions()
	logging.Info(logging.ComponentGCD, "Generating GCD hashtabs for %d versions", len(versions))

	for _, v := range versions {
		if err := s.generateGCD(v.Version); err != nil {
			logging.Error(logging.ComponentGCD, "Failed to generate GCD for version %s: %v", v.Version, err)
		}
	}

	return nil
}

func (s *Service) generateGCD(version string) error {
	hashtabs := s.hashtabService.GetHashtabsForVersion(version)
	if len(hashtabs) == 0 {
		return fmt.Errorf("no hashtabs found for version %s", version)
	}

	if len(hashtabs) == 1 {
		logging.Info(logging.ComponentGCD, "Only one device hashtab for version %s, using it directly", version)
		s.mu.Lock()
		s.gcdHashtabs[version] = &GCDHashtab{
			Version:       version,
			Path:          hashtabs[0].Path,
			SourceModTime: time.Now(),
			DeviceCount:   1,
		}
		s.sourceModTimes[version] = map[string]time.Time{
			hashtabs[0].Path: time.Now(),
		}
		s.mu.Unlock()
		return nil
	}

	outputPath := filepath.Join(s.gcdDir, version+".gcd")

	args := []string{"gcd-hashtab", outputPath}
	for _, ht := range hashtabs {
		args = append(args, ht.Path)
	}

	logging.Info(logging.ComponentGCD, "Generating GCD hashtab for version %s from %d device hashtabs", version, len(hashtabs))

	if err := runQmldiff(s.qmldiffBinary, args...); err != nil {
		return fmt.Errorf("qmldiff gcd-hashtab failed: %w", err)
	}

	s.mu.Lock()
	s.gcdHashtabs[version] = &GCDHashtab{
		Version:       version,
		Path:          outputPath,
		SourceModTime: time.Now(),
		DeviceCount:   len(hashtabs),
	}

	modTimes := make(map[string]time.Time)
	for _, ht := range hashtabs {
		if info, err := os.Stat(ht.Path); err == nil {
			modTimes[ht.Path] = info.ModTime()
		}
	}
	s.sourceModTimes[version] = modTimes
	s.mu.Unlock()

	logging.Info(logging.ComponentGCD, "Generated GCD hashtab for version %s at %s", version, outputPath)

	return nil
}

func (s *Service) GetGCDHashtab(version string) (string, error) {
	reloaded, err := s.hashtabService.CheckAndReload()
	if err != nil {
		logging.Warn(logging.ComponentGCD, "Failed to check hashtab reload: %v", err)
	}

	if reloaded {
		if err := s.generateGCD(version); err != nil {
			return "", err
		}
	} else {
		needsRegen := false

		s.mu.RLock()
		gcd, exists := s.gcdHashtabs[version]
		sourceMods := s.sourceModTimes[version]
		s.mu.RUnlock()

		if !exists {
			needsRegen = true
		} else {
			hashtabs := s.hashtabService.GetHashtabsForVersion(version)
			if len(hashtabs) != gcd.DeviceCount {
				needsRegen = true
			} else {
				for _, ht := range hashtabs {
					if info, err := os.Stat(ht.Path); err == nil {
						if oldMod, ok := sourceMods[ht.Path]; !ok || !oldMod.Equal(info.ModTime()) {
							needsRegen = true
							break
						}
					}
				}
			}
		}

		if needsRegen {
			if err := s.generateGCD(version); err != nil {
				return "", err
			}
		}
	}

	s.mu.RLock()
	gcd, exists := s.gcdHashtabs[version]
	s.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("GCD hashtab not found for version %s", version)
	}

	return gcd.Path, nil
}

func (s *Service) GetVersions() []hashtab.VersionInfo {
	return s.hashtabService.GetVersions()
}
