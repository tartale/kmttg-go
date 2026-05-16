package loaders

import (
	"context"
	"time"

	"github.com/tartale/kmttg-plus/go/pkg/loaders/background"
	"github.com/tartale/kmttg-plus/go/pkg/loaders/beacon"
	"github.com/tartale/kmttg-plus/go/pkg/loaders/cache"
)

func StartAll(ctx context.Context) {
	backgroundDuration := time.Duration(30 * time.Second)
	if ok := cache.LoadAllFilesOnce(); ok {
		backgroundDuration = time.Duration(5 * time.Minute)
	}
	go beacon.Listen(ctx)
	go background.RunLoader(ctx, backgroundDuration)
}
