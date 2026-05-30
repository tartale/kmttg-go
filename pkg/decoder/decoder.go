package decoder

import (
	"context"
	"fmt"
	"io"

	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/decrypter"
)

func Decode(ctx context.Context, in io.Reader, out io.Writer) error {
	if err := decrypter.Decrypt(in, out); err != nil {
		logz.LoggerX.Error(fmt.Errorf("error running decoder: %w", err))
		return err
	}
	return nil
}
