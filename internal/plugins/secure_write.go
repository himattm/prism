package plugins

import (
	"os"

	"github.com/himattm/prism/internal/fsutil"
)

// secureWriteFile securely writes data to a file by creating a temporary file
// in the target directory and atomically renaming it to the target path.
// This prevents symlink attacks and partial writes.
func secureWriteFile(path string, data []byte, perm os.FileMode) error {
	return fsutil.SecureWriteFile(path, data, perm)
}
