package loaders

import (
	"context"

	"github.com/tartale/kmttg-plus/go/pkg/loaders/background"
	"github.com/tartale/kmttg-plus/go/pkg/loaders/beacon"
	"github.com/tartale/kmttg-plus/go/pkg/loaders/cache"
)

func StartAll(ctx context.Context) {
	cache.LoadAllFilesOnce()
	go beacon.Listen(ctx)
	go background.RunLoader(ctx)
}
