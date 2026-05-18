package tivos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"

	"github.com/puzpuzpuz/xsync"
	liberrorz "github.com/tartale/go/pkg/errorz"
	"github.com/tartale/kmttg-plus/go/pkg/apicontext"
	"github.com/tartale/kmttg-plus/go/pkg/client"
	"github.com/tartale/kmttg-plus/go/pkg/config"
	"github.com/tartale/kmttg-plus/go/pkg/errorz"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/model"
	"github.com/tartale/kmttg-plus/go/pkg/shows"
	"go.uber.org/zap"
)

var tivoMap = xsync.NewMapOf[*model.Tivo]()

func Store(tivo *model.Tivo) {
	tivoMap.Store(tivo.Name, tivo)
}

func Load(tivo *model.Tivo) error {
	logz.Logger.Info("Loading shows via RPC", zap.String("tivoName", tivo.Name))
	tivoClient, err := client.NewRpcClient(tivo)
	if err != nil {
		return err
	}

	shows, err := LoadShows(context.Background(), tivoClient)
	if err != nil {
		return err
	}

	newTivo := *tivo
	newTivo.Shows = shows
	tivoMap.Store(tivo.Name, &newTivo)
	logz.Logger.Info("Successfully loaded all shows via RPC", zap.String("tivoName", tivo.Name))
	storeToCache(&newTivo)

	return nil
}

func LoadShows(ctx context.Context, tivoClient *client.TivoClient) ([]model.Show, error) {
	const (
		retryCount = 3
	)

	var (
		shows   []model.Show
		success bool
		err     error
	)

	for range retryCount {
		shows, err = tivoClient.GetShows(ctx)
		if errors.Is(err, errorz.ErrReconnected) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get shows: %w", err)
		}

		success = true
		break
	}
	if !success {
		return nil, fmt.Errorf("failed to get shows; number of retries exceeded: %w", err)
	}

	return shows, nil
}

func List(ctx context.Context) []*model.Tivo {
	var list []*model.Tivo
	tivoFilterFn := apicontext.TivoFilterFn(ctx)
	showFilterFn := apicontext.ShowFilterFn(ctx)
	imageDimensions := apicontext.ShowImageDimensions(ctx)

	tivoMap.Range(func(key string, val *model.Tivo) bool {
		if tivoFilterFn != nil && !tivoFilterFn(val) {
			return true
		}
		tivo := *val
		list = append(list, &tivo)

		tivo.Shows = []model.Show{}
		offsetCountdown := apicontext.ShowOffset(ctx)
		limitCountdown := apicontext.ShowLimit(ctx)
		for _, show := range val.Shows {
			if limitCountdown == 0 {
				break
			}
			if offsetCountdown > 0 {
				offsetCountdown--
				continue
			}
			if showFilterFn != nil && !showFilterFn(show) {
				continue
			}
			show = shows.WithImageURL(show, imageDimensions)
			tivo.Shows = append(tivo.Shows, shows.AsAPIType(show))
			limitCountdown--
		}

		return true
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}

func GetShowForID(recordingID string) (model.Show, error) {
	var result model.Show
	tivoMap.Range(func(key string, val *model.Tivo) bool {
		for _, show := range val.Shows {
			if show.GetID() == recordingID {
				result = shows.Clone(show)
				return false
			}
		}

		return true
	})

	if result == nil {
		return nil, fmt.Errorf("show ID '%s': %w", recordingID, liberrorz.ErrNotFound)
	}

	return result, nil
}

func storeToCache(tivo *model.Tivo) {
	err := os.MkdirAll(config.Values.CacheDir, 0o755)
	if err != nil {
		logz.Logger.Debug("Unable to create cache directory", zap.String("tivoName", tivo.Name), zap.Error(err))
		return
	}
	tivoCacheFile := path.Join(config.Values.CacheDir, tivo.Name+".json")
	data, err := json.MarshalIndent(tivo, "", "  ")
	if err != nil {
		logz.Logger.Debug("Unable to marshal Tivo to JSON; skipping cache write", zap.String("tivoName", tivo.Name), zap.Error(err))
		return
	}
	err = os.WriteFile(tivoCacheFile, data, 0o664)
	if err != nil {
		logz.Logger.Debug("Unable to write cache file", zap.String("tivoName", tivo.Name), zap.Error(err))
		return
	}
	logz.Logger.Debug("Successfully stored all shows to cache", zap.String("tivoName", tivo.Name))
}
