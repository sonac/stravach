package storage

import (
	"database/sql"
	"log/slog"
	"stravach/app/storage/models"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	DB *sql.DB
}

func (s *SQLiteStore) Connect() error {
	db, err := sql.Open("sqlite3", "stravach.db")
	if err != nil {
		slog.Error("cannot open sqlite file")
		return err
	}
	s.DB = db
	return s.createTables()
}

func (s *SQLiteStore) createTables() error {
	userTable := `
    CREATE TABLE IF NOT EXISTS users (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      strava_id INTEGER NOT NULL,
      telegram_chat_id INTEGER NOT NULL,
      username TEXT NOT NULL,
      email TEXT NOT NULL,
      strava_refresh_token TEXT,
      strava_access_code TEXT,
      token_expires_at INTEGER
    );
  `

	userActivityTable := `
    CREATE TABLE IF NOT EXISTS users_activities (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      user_id INTEGER NOT NULL,
      distance REAL,
      moving_time INTEGER,
      elapsed_time INTEGER,
      type TEXT,
      start_date DATETIME,
      average_heartrate REAL,
      average_speed REAL,
      FOREIGN KEY(user_id) REFERENCES users(id)
    );
  `

	_, err := s.DB.Exec(userTable)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(userActivityTable)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLiteStore) GetUserByChatId(chatId int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at FROM users WHERE telegram_chat_id = ?`
	err := s.DB.QueryRow(query, chatId).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessCode, &user.TokenExpiresAt)
	if err != nil {
		return nil, err
	}
	user.Activities, err = s.GetuserActivities(user.ID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) GetuserActivities(userId int64) ([]models.UserActivity, error) {
	activities := []models.UserActivity{}
	query := `SELECT id, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed FROM user_activities WHERE user_id = ?`
	rows, err := s.DB.Query(query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var activity models.UserActivity
		err := rows.Scan(&activity.ID, &activity.Distance, &activity.MovingTime, &activity.ElapsedTime, &activity.ActivityType, &activity.StartDate, &activity.AverageHeartrate, &activity.AverageSpeed)
		if err != nil {
			return nil, err
		}
		activities = append(activities, activity)
	}
	return activities, nil
}

func (s *SQLiteStore) CreateUser(user *models.User) error {
	query := `INSERT INTO users (strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := s.DB.Exec(query, user.StravaId, user.TelegramChatId, user.Username, user.Email, user.StravaRefreshToken, user.StravaAccessToken, user.StravaAccessCode, user.TokenExpiresAt)
	if err != nil {
		return err
	}
	user.ID, err = result.LastInsertId()
	return err
}

func (s *SQLiteStore) CreateUserActivity(activity *models.UserActivity) error {
	query := `INSERT INTO user_activities (user_id, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := s.DB.Exec(query, activity.ID, activity.Distance, activity.MovingTime, activity.ElapsedTime, activity.ActivityType, activity.StartDate, activity.AverageHeartrate, activity.AverageSpeed)
	if err != nil {
		return err
	}
	activity.ID, err = result.LastInsertId()
	return err
}

func (s *SQLiteStore) UpdateUser(user *models.User) error {
	query := `
    UPDATE users
    SET strava_id = ?, telegram_chat_id = ?, username = ?, email = ?,
      strava_refresh_token = ?, strava_access_code = ?, strava_access_code = ?, token_expires_at = ?
    WHERE id = ?
  `
	_, err := s.DB.Exec(query, user.StravaId, user.TelegramChatId, user.Username, user.Email,
		user.StravaRefreshToken, user.StravaAccessToken, user.StravaAccessCode, user.TokenExpiresAt)
	return err
}
