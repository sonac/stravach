package storage

import (
	"stravach/app/storage/models"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStore_CreateUserActivity(t *testing.T) {
	// Setup the mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Initialize the SQLiteStore with the mock database
	sqliteStore := &SQLiteStore{DB: db}

	// Define a UserActivity instance to insert
	activity := &models.UserActivity{
		ID:               1,
		UserID:           123,
		Name:             "Morning Run",
		Distance:         10.5,
		MovingTime:       3600,
		ElapsedTime:      4000,
		ActivityType:     "Run",
		StartDate:        time.Now(),
		AverageHeartrate: 150.5,
		AverageSpeed:     2.9,
		IsUpdated:        false,
	}

	// Define the expected SQL query with the corresponding arguments
	query := `
    INSERT INTO user_activities \(
        id, name, user_id, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed, is_updated
    \) VALUES \(\?, \?, \?, \?, \?, \?, \?, \?, \?, \?, \?\)
    ON CONFLICT\(id\) DO UPDATE SET
        name = excluded.name,
        user_id = excluded.user_id,
        distance = excluded.distance,
        moving_time = excluded.moving_time,
        elapsed_time = excluded.elapsed_time,
        type = excluded.type,
        start_date = excluded.start_date,
        average_heartrate = excluded.average_heartrate,
        average_speed = excluded.average_speed,
        is_updated = excluded.is_updated
`
	// Mock the expected result from Exec
	mock.ExpectExec(query).
		WithArgs(activity.ID, activity.Name, activity.UserID, activity.Distance, activity.MovingTime, activity.ElapsedTime, activity.ActivityType, activity.StartDate, activity.AverageHeartrate, activity.AverageSpeed, activity.IsUpdated).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call the function
	err = sqliteStore.CreateUserActivity(activity, activity.UserID)
	require.NoError(t, err)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}
