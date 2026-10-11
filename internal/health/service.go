package health

import (
	"context"
	"database/sql"
	"time"
)

type Status string

const (
	Healthy  Status = "healthy"
	Degraded Status = "degraded"
	Failed   Status = "failed"
)

type Result struct {
	Application  Status   `json:"application"`
	Database     Status   `json:"database"`
	Integrations Status   `json:"integrations"`
	Devices      []Device `json:"devices"`
}

type Device struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`
	PropertyName  string     `json:"property_name"`
	Location      *string    `json:"location,omitempty"`
	Status        string     `json:"status"`
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	LastError     *string    `json:"last_error,omitempty"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CheckApplication(ctx context.Context) Result {
	result := Result{
		Application:  Healthy,
		Database:     Healthy,
		Integrations: Healthy,
		Devices:      []Device{},
	}

	if err := s.db.PingContext(ctx); err != nil {
		result.Database = Failed
		result.Application = Degraded
	}

	return result
}

func (s *Service) Check(ctx context.Context, propertyID int) Result {
	result := s.CheckApplication(ctx)

	if result.Database == Failed {
		return result
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
		SELECT
			d.id,
			d.name,
			d.type,
			p.name,
			d.location,
			COALESCE(d.status, 'unknown'),
			d.last_heartbeat,
			d.last_error
		FROM devices d
		JOIN properties p ON p.id = d.property_id
		WHERE d.property_id = $1
		ORDER BY d.name
		`,
		propertyID,
	)
	if err != nil {
		result.Integrations = Degraded
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var item Device
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Type,
			&item.PropertyName,
			&item.Location,
			&item.Status,
			&item.LastHeartbeat,
			&item.LastError,
		); err != nil {
			result.Integrations = Degraded
			continue
		}

		result.Devices = append(result.Devices, item)
	}

	if err := rows.Err(); err != nil {
		result.Integrations = Degraded
	}

	return result
}
