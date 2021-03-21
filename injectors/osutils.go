package injectors

import (
	"fmt"
	"io/fs"
	"os"
)

func checkIfExistsAndIsRegular(path string) (fs.FileInfo, error) {
	stat, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr // don't wrap OS errors:
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: '%s'", path)
	}
	return stat, nil
}

func pathExists(path string) (bool, error) {
	_, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, statErr
	}
	// something exists:
	return true, nil
}
