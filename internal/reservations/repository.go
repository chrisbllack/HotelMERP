package reservations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("reservation not found")
var ErrConflict = errors.New("reservation conflict")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	propertyID int,
	req CreateRequest,
	operationID uuid.UUID,
) (*Reservation, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, fmt.Errorf("begin reservation transaction: %w", err)
	}
	defer tx.Rollback()

	var roomID interface{}
	if req.RoomID != nil {
		roomID = *req.RoomID
	}

	var roomTypeID interface{}
	if req.RoomTypeID != nil {
		roomTypeID = *req.RoomTypeID
	}

	var candidateRoomID int
	err = tx.QueryRowContext(
		ctx,
		`
		SELECT r.id
		FROM rooms r
		WHERE r.property_id = $1
		  AND ($2::integer IS NULL OR r.id = $2)
		  AND ($3::integer IS NULL OR r.room_type_id = $3)
		  AND COALESCE(r.deleted_at, NULL) IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM reservations x
			WHERE x.property_id = r.property_id
			  AND x.room_id = r.id
			  AND x.status IN ('tentative', 'confirmed', 'checked_in')
			  AND x.arrival_date < $5::date
			  AND x.departure_date > $4::date
			  AND COALESCE(x.deleted_at, NULL) IS NULL
		  )
		ORDER BY r.id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
		`,
		propertyID,
		roomID,
		roomTypeID,
		req.ArrivalDate,
		req.DepartureDate,
	).Scan(&candidateRoomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("check reservation availability: %w", err)
	}

	var item Reservation
	err = tx.QueryRowContext(
		ctx,
		`
		INSERT INTO reservations (
			guest_id,
			room_id,
			room_type_id,
			property_id,
			confirmation_number,
			arrival_date,
			departure_date,
			status,
			source,
			operation_id,
			created_at,
			updated_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			'HP-' || UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text, '-', '') FROM 1 FOR 8)),
			$5::date,
			$6::date,
			'tentative',
			COALESCE(NULLIF($7, ''), 'direct'),
			$8,
			NOW(),
			NOW()
		)
		RETURNING
			id,
			property_id,
			guest_id,
			room_id,
			room_type_id,
			confirmation_number,
			arrival_date::text,
			departure_date::text,
			status,
			source,
			COALESCE(balance_minor, 0),
			COALESCE(payment_status, 'unpaid'),
			created_at,
			updated_at
		`,
		req.GuestID,
		candidateRoomID,
		req.RoomTypeID,
		propertyID,
		req.ArrivalDate,
		req.DepartureDate,
		req.Source,
		operationID,
	).Scan(
		&item.ID,
		&item.PropertyID,
		&item.GuestID,
		&item.RoomID,
		&item.RoomTypeID,
		&item.ConfirmationNumber,
		&item.ArrivalDate,
		&item.DepartureDate,
		&item.Status,
		&item.Source,
		&item.BalanceMinor,
		&item.PaymentStatus,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert reservation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reservation: %w", err)
	}

	return &item, nil
}

func (r *Repository) Get(
	ctx context.Context,
	propertyID int,
	id int,
) (*Reservation, error) {
	var item Reservation

	err := r.db.QueryRowContext(
		ctx,
		`
		SELECT
			id,
			property_id,
			guest_id,
			room_id,
			room_type_id,
			confirmation_number,
			arrival_date::text,
			departure_date::text,
			status,
			COALESCE(source, 'direct'),
			COALESCE(balance_minor, 0),
			COALESCE(payment_status, 'unpaid'),
			created_at,
			updated_at
		FROM reservations
		WHERE property_id = $1
		  AND id = $2
		  AND COALESCE(deleted_at, NULL) IS NULL
		`,
		propertyID,
		id,
	).Scan(
		&item.ID,
		&item.PropertyID,
		&item.GuestID,
		&item.RoomID,
		&item.RoomTypeID,
		&item.ConfirmationNumber,
		&item.ArrivalDate,
		&item.DepartureDate,
		&item.Status,
		&item.Source,
		&item.BalanceMinor,
		&item.PaymentStatus,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get reservation: %w", err)
	}

	return &item, nil
}

func (r *Repository) RecordStatusHistory(
	ctx context.Context,
	propertyID int,
	reservationID int,
	from Status,
	to Status,
	userID int,
	reason string,
) error {
	_, err := r.db.ExecContext(
		ctx,
		`
		INSERT INTO reservation_status_history (
			reservation_id,
			property_id,
			from_status,
			to_status,
			reason,
			changed_by,
			created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		`,
		reservationID,
		propertyID,
		from,
		to,
		reason,
		userID,
		time.Now().UTC(),
	)
	return err
}
