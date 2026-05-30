package decrypter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tartale/go/pkg/asserts"
)

func TestDecrypter(t *testing.T) {
	mediaAccessKey := os.Getenv("KMTTG_MEDIA_ACCESS_KEY")
	testDataDir := os.Getenv("KMTTG_TEST_DATA_DIR")
	tmpDir := os.Getenv("KMTTG_TEMP_DIR")
	if mediaAccessKey == "" || testDataDir == "" || tmpDir == "" {
		t.Skip("Skipping test: requires KMTTG_MEDIA_ACCESS_KEY, KMTTG_TEST_DATA_DIR, and KMTTG_TEMP_DIR environment variables to be set")
	}

	encryptedFiles, err := filepath.Glob(filepath.Join(testDataDir, "*-encrypted.TiVo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(encryptedFiles) == 0 {
		t.Skip("no *-encrypted.TiVo files found in", testDataDir)
	}

	for _, inputFilePath := range encryptedFiles {
		base := strings.TrimSuffix(filepath.Base(inputFilePath), "-encrypted.TiVo")
		t.Run(base, func(t *testing.T) {
			expectedFilePath := filepath.Join(testDataDir, base+"-decrypted.ts")
			if _, err := os.Stat(expectedFilePath); err != nil {
				t.Skipf("no matching decrypted file for %s", inputFilePath)
			}
			outputFilePath := filepath.Join(tmpDir, base+"-actual.ts")

			input, err := os.Open(inputFilePath)
			assert.NoError(t, err)
			defer input.Close()

			output, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o664)
			assert.NoError(t, err)
			defer output.Close()

			err = Decrypt(input, output)
			assert.NoError(t, err)
			output.Close()

			output, err = os.Open(outputFilePath)
			assert.NoError(t, err)
			defer output.Close()

			expected, err := os.Open(expectedFilePath)
			assert.NoError(t, err)
			defer expected.Close()

			expectedStat, err := os.Stat(expectedFilePath)
			assert.NoError(t, err)
			outputStat, err := os.Stat(outputFilePath)
			assert.NoError(t, err)
			assert.Equal(t, expectedStat.Size(), outputStat.Size())
			asserts.ReadersEqual(t, expected, output)
		})
	}
}
