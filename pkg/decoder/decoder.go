package decoder

import (
	"context"
	"fmt"
	"io"

	"github.com/tartale/kmttg-plus/go/pkg/config"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/tivolibre"
)

func Decode(ctx context.Context, in io.Reader, out io.Writer) error {
	logz.LoggerX.Debugf("Start decoding")
	decoder := tivolibre.NewDecoder(config.Values.MediaAccessKey)
	if err := decoder.Decode(in, out); err != nil {
		logz.LoggerX.Error(fmt.Errorf("error running decoder: %w", err))
		return err
	}
	logz.LoggerX.Debug("Finished decoding")
	return nil
}
