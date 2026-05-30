package encoder

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/tartale/kmttg-plus/go/pkg/config"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
)

func Encode(input io.Reader, outputPath string) error {
	ffmpeg := filepath.Join(config.Values.ToolsDir, "ffmpeg")
	cmd := exec.Command(ffmpeg,
		"-i", "pipe:0",
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-f", "matroska",
		outputPath,
	)
	cmd.Stdin = input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	logz.LoggerX.Debugf("Encode command: %s", cmd.String())

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, stderr.String())
	}

	return nil
}
