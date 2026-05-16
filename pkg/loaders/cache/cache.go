package cache

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tartale/go/pkg/filez"
	"github.com/tartale/kmttg-plus/go/pkg/config"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/model"
	"github.com/tartale/kmttg-plus/go/pkg/shows"
	"github.com/tartale/kmttg-plus/go/pkg/tivos"
	"go.uber.org/zap"
)

var loadFromCacheOnce sync.Once

func LoadAllFilesOnce() bool {
	cacheLoadSuccessful := false
	loadFromCacheOnce.Do(func() {
		cachedTivos := loadAllFiles()
		if len(cachedTivos) > 0 {
			cacheLoadSuccessful = true
		}
		for _, tivo := range cachedTivos {
			tivos.Store(tivo)
		}
	})

	return cacheLoadSuccessful
}

func loadAllFiles() []*model.Tivo {
	var cached []*model.Tivo

	filepath.WalkDir(config.Values.CacheDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if filez.IsDir(filePath) {
			return nil
		}
		if !strings.HasSuffix(filePath, ".json") {
			return nil
		}

		tivo := model.Tivo{
			Name: filez.NameWithoutExtension(filePath),
		}
		if loaded := loadFromCache(&tivo); !loaded {
			logz.Logger.Warn("Cache could not be loaded; removing", zap.String("tivoName", tivo.Name))
			filez.MustRemoveAll(filePath)
			return nil
		}

		cached = append(cached, &tivo)
		return nil
	})

	return cached
}

func loadFromCache(tivo *model.Tivo) bool {
	tivoCacheFile := path.Join(config.Values.CacheDir, tivo.Name+".json")
	if _, err := os.Stat(tivoCacheFile); errors.Is(err, os.ErrNotExist) {
		logz.Logger.Debug("No cache found", zap.String("tivoName", tivo.Name))
		return false
	}
	logz.Logger.Debug("Loading shows from cache", zap.String("tivoName", tivo.Name))
	data, err := os.ReadFile(tivoCacheFile)
	if err != nil {
		logz.Logger.Debug("Unable to load cache file", zap.String("tivoName", tivo.Name), zap.Error(err))
		return false
	}
	var aux struct {
		Name    string            `json:"name"`
		Address string            `json:"address"`
		Tsn     string            `json:"tsn"`
		Shows   []json.RawMessage `json:"shows,omitempty"`
	}
	err = json.Unmarshal(data, &aux)
	if err != nil {
		logz.Logger.Debug("Unable to load cache file", zap.String("tivoName", tivo.Name), zap.Error(err))
		return false
	}
	newTivo := model.Tivo{
		Name:    aux.Name,
		Address: aux.Address,
		Tsn:     aux.Tsn,
		Shows:   make([]model.Show, 0, len(aux.Shows)),
	}
	for _, raw := range aux.Shows {
		show, err := shows.UnmarshalShowFromJSON(raw, &newTivo)
		if err != nil {
			logz.Logger.Debug("Unable to unmarshal show from cache", zap.String("tivoName", tivo.Name), zap.Error(err))
			return false
		}
		newTivo.Shows = append(newTivo.Shows, show)
	}
	logz.Logger.Debug("Successfully loaded all shows from cache", zap.String("tivoName", tivo.Name))
	*tivo = newTivo
	return true
}
