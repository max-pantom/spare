package doctor

import (
	"os"

	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/profile"
)

func storageChecks(instance model.Instance) []Check {
	info, err := os.Stat(instance.DataPath)
	if err != nil || !info.IsDir() {
		return []Check{{
			ID:       "storage." + instance.ID,
			Name:     "Selected folder",
			Status:   "failed",
			Message:  "The selected folder is unavailable.",
			Recovery: "Reconnect or restore the folder, then start the recipe again.",
		}}
	}
	result := []Check{{
		ID:      "storage." + instance.ID,
		Name:    "Selected folder",
		Status:  "healthy",
		Message: "The selected folder is readable.",
	}}
	if instance.RecipeID == model.RecipeDrop {
		if writable(instance.DataPath) {
			result = append(result, Check{
				ID:      "storage-write." + instance.ID,
				Name:    "Destination access",
				Status:  "healthy",
				Message: "Drop can write to the selected folder.",
			})
		} else {
			result = append(result, Check{
				ID:       "storage-write." + instance.ID,
				Name:     "Destination access",
				Status:   "failed",
				Message:  "Drop cannot write to the selected folder.",
				Recovery: "Choose a writable folder or restore its permissions.",
			})
		}
	}
	available := profile.StorageAvailable(instance.DataPath)
	if available == 0 {
		result = append(result, Check{
			ID:       "storage-free." + instance.ID,
			Name:     "Available storage",
			Status:   "warning",
			Message:  "Available storage could not be measured.",
			Recovery: "Check that the selected folder is mounted and readable.",
		})
		return result
	}
	status := "healthy"
	recovery := ""
	if available > 0 && available < 1024*1024*1024 {
		status = "warning"
		recovery = "Free storage space before receiving more files."
	}
	result = append(result, Check{
		ID:       "storage-free." + instance.ID,
		Name:     "Available storage",
		Status:   status,
		Message:  formatBytes(available) + " is available in the selected folder.",
		Recovery: recovery,
	})
	return result
}

func formatBytes(value uint64) string {
	const gib = uint64(1024 * 1024 * 1024)
	if value >= gib {
		return formatNumber(float64(value)/float64(gib)) + " GB"
	}
	return formatNumber(float64(value)/(1024*1024)) + " MB"
}

func formatNumber(value float64) string {
	if value >= 10 {
		return integerFormat(int64(value))
	}
	return oneDecimal(value)
}
