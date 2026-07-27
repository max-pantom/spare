package backup

import (
	"errors"

	"github.com/spare-run/spare/internal/model"
)

func ExportInstance(instance model.Instance, destination string) error {
	if instance.DataPath == "" && instance.RootPath == "" {
		return errors.New("this recipe has no selected-folder data to export")
	}
	if instance.DataPath == "" {
		return Export(instance.RootPath, fromInstance(instance), destination)
	}
	return Export(instance.DataPath, fromInstance(instance), destination)
}
