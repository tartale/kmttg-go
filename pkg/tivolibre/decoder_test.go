package tivolibre

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tartale/kmttg-plus/go/pkg/config"
)

func TestDecoder(t *testing.T) {
	mediaAccessKey := os.Getenv("KMTTG_MEDIA_ACCESS_KEY")
	testDataDir := os.Getenv("KMTTG_TEST_DATA_DIR")
	if mediaAccessKey == "" || testDataDir == "" {
		t.Skip("Skipping test: requires KMTTG_MEDIA_ACCESS_KEY and KMTTG_TEST_DATA_DIR environment variables to be set")
	}
	decoder := NewDecoder(config.Values.MediaAccessKey)
	input, err := os.Open(filepath.Join(testDataDir, "encrypted.TiVo"))
	assert.NoError(t, err)
	defer input.Close()
	var outputBytes bytes.Buffer
	outputWriter := bufio.NewWriter(&outputBytes)
	err = decoder.Decode(input, outputWriter)
	assert.NoError(t, err)
	assert.NotEmpty(t, outputBytes)
	assert.Greater(t, outputBytes.Len(), 0)
	expected, err := os.Open(filepath.Join(testDataDir, "decrypted.ts"))
	assert.NoError(t, err)
	defer expected.Close()
	// assert.Equal(t, expected, outputBytes)
}
