package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"stravach/app/storage/models"

	_ "github.com/mattn/go-sqlite3"
)

type Store interface {
	Connect() error
	GetActivityById(activityId int64) (*models.UserActivity, error)
	CreateUserActivity(activity *models.UserActivity, userId int64) error
	GetUserActivities(userId int64) ([]models.UserActivity, error)
	UpdateUser(user *models.User) error
	GetUserByStravaId(stravaId int64) (*models.User, error)
	GetUserByChatId(chatId int64) (*models.User, error)
	GetUserById(id int64) (*models.User, error)
	IsActivityExists(activityId int64) (bool, error)
}

var _ Store = (*SQLiteStore)(nil)

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
		      strava_id INTEGER UNIQUE NOT NULL,
		      telegram_chat_id INTEGER UNIQUE NOT NULL,
		      username TEXT NOT NULL,
		      email TEXT NOT NULL,
		      strava_refresh_token TEXT,
		      strava_access_token TEXT,
		      strava_access_code TEXT,
		      token_expires_at INTEGER
		    );
		  `
	userActivityTable := `
    CREATE TABLE IF NOT EXISTS user_activities (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      name VARCHAR,
      user_id INTEGER NOT NULL,
      distance REAL,
      moving_time INTEGER,
      elapsed_time INTEGER,
      type TEXT,
      start_date DATETIME,
      average_heartrate REAL,
      average_speed REAL,
      is_updated INTEGER DEFAULT 0,
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

func (s *SQLiteStore) CreateUser(user *models.User) error {
	query := `
		INSERT INTO users (
				strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(strava_id) DO UPDATE SET
				telegram_chat_id = excluded.telegram_chat_id,
				username = excluded.username,
				email = excluded.email,
				strava_refresh_token = excluded.strava_refresh_token,
				strava_access_token = excluded.strava_access_token,
				strava_access_code = excluded.strava_access_code,
				token_expires_at = excluded.token_expires_at
	`
	result, err := s.DB.Exec(query, user.StravaId, user.TelegramChatId, user.Username, user.Email, user.StravaRefreshToken, user.StravaAccessToken, user.StravaAccessCode, user.TokenExpiresAt)
	if err != nil {
		slog.Error("error while creating user")
		return err
	}
	user.ID, err = result.LastInsertId()
	return err
}

func (s *SQLiteStore) GetUserByChatId(chatId int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at FROM users WHERE telegram_chat_id = ?`
	err := s.DB.QueryRow(query, chatId).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessToken, &user.StravaAccessCode, &user.TokenExpiresAt)
	if err != nil {
		slog.Error("error while fetching user chat by id", "id", chatId)
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) GetUserById(id int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at FROM users WHERE id = ?`
	fmt.Println(id)
	err := s.DB.QueryRow(query, id).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessToken, &user.StravaAccessCode, &user.TokenExpiresAt)
	if err != nil {
		slog.Error("error while fetching user by id", "id", id)
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) GetUserByStravaId(id int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at FROM users WHERE strava_id = ?`
	err := s.DB.QueryRow(query, id).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessToken, &user.StravaAccessCode, &user.TokenExpiresAt)
	if err != nil {
		slog.Error("error while fetching user by strava id", "id", id)
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) IsUserExistsByChatId(chatId int64) (bool, error) {
	var exists bool
	query := `SELECT COUNT(1) FROM users WHERE id = ?`
	err := s.DB.QueryRow(query, chatId).Scan(&exists)
	if err != nil {
		slog.Error("error while checking if activity exists", "id", chatId)
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) UpdateUser(user *models.User) error {
	slog.Debug("updating user", "usr", fmt.Sprintf("%+v", user))
	query := `
    UPDATE users
    SET strava_id = ?, telegram_chat_id = ?, username = ?, email = ?,
      strava_refresh_token = ?, strava_access_token = ?, strava_access_code = ?, token_expires_at = ?
    WHERE id = ?
  `
	_, err := s.DB.Exec(query, user.StravaId, user.TelegramChatId, user.Username, user.Email,
		user.StravaRefreshToken, user.StravaAccessToken, user.StravaAccessCode, user.TokenExpiresAt, user.ID)
	return err
}

func (s *SQLiteStore) CreateUserActivities(userId int64, activities *[]models.UserActivity) error {
	query := `
    INSERT INTO user_activities (
        id, name, user_id, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed, is_updated
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(id) DO UPDATE SET
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
	for _, a := range *activities {
		_, err := s.DB.Exec(query, a.ID, a.Name, userId, a.Distance, a.MovingTime, a.ElapsedTime, a.ActivityType, a.StartDate, a.AverageHeartrate, a.AverageSpeed, a.IsUpdated)
		if err != nil {
			slog.Error("error while creating user activities")
			return err
		}
	}
	slog.Info(fmt.Sprintf("inserted %d activities", len(*activities)))
	return nil
}

func (s *SQLiteStore) CreateUserActivity(activity *models.UserActivity, userId int64) error {
	query := `
    INSERT INTO user_activities (
        id, name, user_id, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed, is_updated
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(id) DO UPDATE SET
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
	result, err := s.DB.Exec(query, activity.ID, activity.Name, userId, activity.Distance, activity.MovingTime, activity.ElapsedTime, activity.ActivityType, activity.StartDate, activity.AverageHeartrate, activity.AverageSpeed, activity.IsUpdated)
	if err != nil {
		slog.Error("error while creating user activitiy")
		return err
	}
	activity.ID, err = result.LastInsertId()
	return err
}

func (s *SQLiteStore) GetUserActivities(userId int64) ([]models.UserActivity, error) {
	var activities []models.UserActivity
	query := `SELECT id, name, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed, is_updated FROM user_activities WHERE user_id = ?`
	rows, err := s.DB.Query(query, userId)
	if err != nil {
		slog.Error("error while fetching user activities", "id", userId)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var activity models.UserActivity
		err := rows.Scan(&activity.ID, &activity.Name, &activity.Distance, &activity.MovingTime, &activity.ElapsedTime, &activity.ActivityType, &activity.StartDate, &activity.AverageHeartrate, &activity.AverageSpeed, &activity.IsUpdated)
		if err != nil {
			return nil, err
		}
		activities = append(activities, activity)
	}
	return activities, nil
}

func (s *SQLiteStore) GetActivityById(activityId int64) (*models.UserActivity, error) {
	activity := models.UserActivity{}
	query := `SELECT id, name, user_id, distance, moving_time, elapsed_time, type, start_date, average_heartrate, average_speed, is_updated FROM user_activities WHERE id = ?`
	err := s.DB.QueryRow(query, activityId).Scan(&activity.ID, &activity.Name, &activity.UserID, &activity.Distance, &activity.MovingTime, &activity.ElapsedTime, &activity.ActivityType, &activity.StartDate, &activity.AverageHeartrate, &activity.AverageSpeed, &activity.IsUpdated)
	if err != nil {
		slog.Error("error while fetching user activity", "id", activityId)
		return nil, err
	}
	return &activity, nil
}

func (s *SQLiteStore) IsActivityExists(activityId int64) (bool, error) {
	var exists bool
	query := `SELECT COUNT(1) FROM user_activities WHERE id = ?`
	err := s.DB.QueryRow(query, activityId).Scan(&exists)
	if err != nil {
		slog.Error("error while checking if activity exists", "id", activityId)
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) UpdateUserActivity(activity *models.UserActivity) error {
	fmt.Printf("%+v", activity)
	query := `
    UPDATE user_activities
    SET
        name = ?,
        distance = ?,
        moving_time = ?,
        elapsed_time = ?,
        type = ?,
        start_date = ?,
        average_heartrate = ?,
        average_speed = ?,
        is_updated = ?
    WHERE id = ?
  `
	result, err := s.DB.Exec(query, activity.Name, activity.Distance, activity.MovingTime,
		activity.ElapsedTime, activity.ActivityType, activity.StartDate, activity.AverageHeartrate, activity.AverageSpeed, activity.IsUpdated, activity.ID)
	if err != nil {
		slog.Error("error while updating user activity")
		return err
	}
	slog.Debug("activity updated")
	_, err = result.LastInsertId()
	return err
}
