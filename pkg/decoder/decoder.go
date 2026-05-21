package decoder

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/tartale/kmttg-plus/go/pkg/config"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
)

func Decode(ctx context.Context, in io.Reader, out io.Writer) error {
	decoderCommand := strings.Split(config.Values.TivoDecodeCmd, " ")
	decoder := exec.CommandContext(ctx, decoderCommand[0], decoderCommand[1:]...)
	decoder.Stdin = in
	decoder.Stdout = out
	logz.LoggerX.Debugf("Start decoding")
	if err := decoder.Run(); err != nil {
		logz.LoggerX.Error(fmt.Errorf("error running decoder: %w", err))
		return err
	}
	logz.LoggerX.Debug("Finished decoding")
	return nil
}
