package background

import (
	"context"
	"time"

	"github.com/tartale/kmttg-plus/go/pkg/config"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/tivos"
	"go.uber.org/zap"
)

var LoadTicker = time.NewTicker(config.Values.ReloadInterval)

func RunLoader(ctx context.Context) {
	for range LoadTicker.C {
		err := tivos.LoadAll(ctx)
		if err != nil {
			logz.Logger.Warn("Error loading shows", zap.Error(err))
			LoadTicker.Reset(30 * time.Second)
		} else {
			LoadTicker.Reset(config.Values.ReloadInterval)
		}
	}
}
