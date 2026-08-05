package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/spare-run/spare/internal/model"
)

func (s *Store) PutJobPackage(ctx context.Context, value model.JobPackage) error {
	if value.InstalledAt.IsZero() {
		value.InstalledAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO job_packages(
			id, version, publisher, minimum_spare_version, checksum, signature,
			signature_status, manifest_json, package_path, source, installed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			publisher = excluded.publisher,
			minimum_spare_version = excluded.minimum_spare_version,
			checksum = excluded.checksum,
			signature = excluded.signature,
			signature_status = excluded.signature_status,
			manifest_json = excluded.manifest_json,
			package_path = excluded.package_path,
			source = excluded.source,
			installed_at = excluded.installed_at
	`, value.ID, value.Version, value.Publisher, value.MinimumSpare,
		value.Checksum, value.Signature, value.SignatureStatus,
		value.ManifestJSON, value.PackagePath, value.Source,
		value.InstalledAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) JobPackage(ctx context.Context, id string) (model.JobPackage, error) {
	var value model.JobPackage
	var installedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, version, publisher, minimum_spare_version, checksum, signature,
			signature_status, manifest_json, package_path, source, installed_at
		FROM job_packages WHERE id = ?
	`, id).Scan(
		&value.ID, &value.Version, &value.Publisher, &value.MinimumSpare,
		&value.Checksum, &value.Signature, &value.SignatureStatus,
		&value.ManifestJSON, &value.PackagePath, &value.Source, &installedAt,
	)
	if err != nil {
		return model.JobPackage{}, err
	}
	value.InstalledAt, _ = time.Parse(time.RFC3339Nano, installedAt)
	return value, nil
}

func (s *Store) JobPackages(ctx context.Context) ([]model.JobPackage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, version, publisher, minimum_spare_version, checksum, signature,
			signature_status, manifest_json, package_path, source, installed_at
		FROM job_packages ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.JobPackage
	for rows.Next() {
		var value model.JobPackage
		var installedAt string
		if err := rows.Scan(
			&value.ID, &value.Version, &value.Publisher, &value.MinimumSpare,
			&value.Checksum, &value.Signature, &value.SignatureStatus,
			&value.ManifestJSON, &value.PackagePath, &value.Source, &installedAt,
		); err != nil {
			return nil, err
		}
		value.InstalledAt, _ = time.Parse(time.RFC3339Nano, installedAt)
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) DeleteJobPackage(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM job_packages WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) PutJobProfile(ctx context.Context, profile model.JobProfile) error {
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now().UTC()
	}
	configJSON, err := json.Marshal(profile.Config)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO job_profiles(recipe_id, config_json, port, port_mode, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(recipe_id) DO UPDATE SET
			config_json = excluded.config_json,
			port = excluded.port,
			port_mode = excluded.port_mode,
			updated_at = excluded.updated_at
	`, profile.RecipeID, configJSON, profile.Port, profile.PortMode,
		profile.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) JobProfile(ctx context.Context, recipeID string) (model.JobProfile, error) {
	var profile model.JobProfile
	var configJSON []byte
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT recipe_id, config_json, port, port_mode, updated_at
		FROM job_profiles WHERE recipe_id = ?
	`, recipeID).Scan(
		&profile.RecipeID, &configJSON, &profile.Port, &profile.PortMode, &updatedAt,
	)
	if err != nil {
		return model.JobProfile{}, err
	}
	if err := json.Unmarshal(configJSON, &profile.Config); err != nil {
		return model.JobProfile{}, err
	}
	profile.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return profile, nil
}
