package background

import (
	"context"
	"errors"
	"time"

	"github.com/tartale/go/pkg/errorx"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/tivos"
	"go.uber.org/zap"
)

func RunLoader(ctx context.Context, initialDuration time.Duration) {
	loadAllTicker := time.NewTicker(initialDuration)

	for range loadAllTicker.C {
		err := LoadAll(ctx)
		if err != nil {
			logz.Logger.Warn("Error loading shows", zap.Error(err))
			loadAllTicker.Reset(30 * time.Second)
		} else {
			loadAllTicker.Reset(5 * time.Minute)
		}
	}
}

func LoadAll(ctx context.Context) error {
	var errs errorx.Errors
	tivoList := tivos.List(ctx)
	if len(tivoList) == 0 {
		return errors.New("no TiVos found")
	}
	for _, tivo := range tivoList {
		errs = append(errs, tivos.Load(tivo))
	}

	return errs.Combine("errors when loading shows", "\n")
}
