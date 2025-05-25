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
	db, err := sql.Open("sqlite3", "stravach_remote.db")
	if err != nil {
		slog.Error("cannot open sqlite file")
		return err
	}
	s.DB = db
	if err = s.createTables(); err != nil {
		slog.Error("cannot create tables", "err", err)
		return err
	}
	if err = s.migrate(); err != nil {
		slog.Error("cannot migrate tables", "err", err)
		return err
	}

	return nil
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
		      token_expires_at INTEGER,
			  language TEXT
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

func (s *SQLiteStore) migrate() error {
	query := `PRAGMA table_info(users);`
	rows, err := s.DB.Query(query)
	if err != nil {
		return fmt.Errorf("failed to retrieve table info: %w", err)
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	type colInfo struct {
		name    string
		notnull int
	}
	var cols []colInfo
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		cols = append(cols, colInfo{name, notnull})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate rows: %w", err)
	}

	var needMigration bool
	for _, col := range cols {
		if (col.name == "strava_id" || col.name == "email") && col.notnull == 1 {
			needMigration = true
		}
	}

	if needMigration {
		slog.Info("Migrating users table to relax NOT NULL on strava_id and email...")
		_, err := s.DB.Exec(`
			CREATE TABLE IF NOT EXISTS users_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				strava_id INTEGER UNIQUE,
				telegram_chat_id INTEGER UNIQUE NOT NULL,
				username TEXT NOT NULL,
				email TEXT,
				strava_refresh_token TEXT,
				strava_access_token TEXT,
				strava_access_code TEXT,
				token_expires_at INTEGER,
				language TEXT
			);
		`)
		if err != nil {
			return fmt.Errorf("failed to create users_new table: %w", err)
		}
		_, err = s.DB.Exec(`
			INSERT INTO users_new (id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at, language)
			SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at, language FROM users;
		`)
		if err != nil {
			return fmt.Errorf("failed to copy data to users_new: %w", err)
		}
		_, err = s.DB.Exec(`DROP TABLE users;`)
		if err != nil {
			return fmt.Errorf("failed to drop old users table: %w", err)
		}
		_, err = s.DB.Exec(`ALTER TABLE users_new RENAME TO users;`)
		if err != nil {
			return fmt.Errorf("failed to rename users_new to users: %w", err)
		}
		slog.Info("Migration completed: users table now allows NULL for strava_id and email.")
	}

	// Also ensure 'language' column exists (legacy logic)
	langColExists := false
	for _, col := range cols {
		if col.name == "language" {
			langColExists = true
			break
		}
	}
	if !langColExists {
		alterQuery := `ALTER TABLE users ADD COLUMN language TEXT DEFAULT 'English'`
		_, err := s.DB.Exec(alterQuery)
		if err != nil {
			return fmt.Errorf("failed to alter users language: %w", err)
		}
		slog.Info("Added 'language' column to users table")
	} else {
		slog.Info("All new columns are present")
	}
	return nil
}

func (s *SQLiteStore) CreateUser(user *models.User) error {
	slog.Info("inserting user", "user_struct", fmt.Sprintf("%+v", user))
	username := "anonymous"
	if user.Username != "" {
		username = user.Username
	}
	slog.Info("CreateUser values", "strava_id", user.StravaId, "telegram_chat_id", user.TelegramChatId, "username", username, "email", user.Email, "strava_refresh_token", user.StravaRefreshToken, "strava_access_token", user.StravaAccessToken, "strava_access_code", user.StravaAccessCode, "token_expires_at", user.TokenExpiresAt, "language", user.Language)
	query := `
		INSERT INTO users (
				strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at, language
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(telegram_chat_id) DO UPDATE SET
				telegram_chat_id = excluded.telegram_chat_id,
				username = username,
				email = excluded.email,
				strava_refresh_token = excluded.strava_refresh_token,
				strava_access_token = excluded.strava_access_token,
				strava_access_code = excluded.strava_access_code,
				token_expires_at = excluded.token_expires_at,
				language = excluded.language
	`
	result, err := s.DB.Exec(query, user.StravaId, user.TelegramChatId, username, user.Email, user.StravaRefreshToken, user.StravaAccessToken, user.StravaAccessCode, user.TokenExpiresAt, "English")
	if err != nil {
		slog.Error("error while creating user", "err", err, "strava_id", user.StravaId, "telegram_chat_id", user.TelegramChatId, "username", username, "email", user.Email)
		return err
	}
	user.ID, err = result.LastInsertId()
	return err
}

func (s *SQLiteStore) GetUserByChatId(chatId int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at, language FROM users WHERE telegram_chat_id = ?`
	err := s.DB.QueryRow(query, chatId).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessToken, &user.StravaAccessCode, &user.TokenExpiresAt, &user.Language)
	if err != nil {
		slog.Error("error while fetching user chat by id", "id", chatId)
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) GetUserById(id int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at, language FROM users WHERE id = ?`
	fmt.Println(id)
	err := s.DB.QueryRow(query, id).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessToken, &user.StravaAccessCode, &user.TokenExpiresAt, &user.Language)
	if err != nil {
		slog.Error("error while fetching user by id", "id", id)
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) GetUserByStravaId(id int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, strava_id, telegram_chat_id, username, email, strava_refresh_token, strava_access_token, strava_access_code, token_expires_at, language FROM users WHERE strava_id = ?`
	err := s.DB.QueryRow(query, id).Scan(&user.ID, &user.StravaId, &user.TelegramChatId, &user.Username, &user.Email, &user.StravaRefreshToken, &user.StravaAccessToken, &user.StravaAccessCode, &user.TokenExpiresAt, &user.Language)
	if err != nil {
		slog.Error("error while fetching user by strava id", "id", id)
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStore) IsUserExistsByChatId(chatId int64) (bool, error) {
	var exists bool
	query := `SELECT COUNT(1) FROM users WHERE telegram_chat_id = ?`
	err := s.DB.QueryRow(query, chatId).Scan(&exists)
	if err != nil {
		slog.Error("error while checking if activity exists", "id", chatId)
		return false, err
	}
	slog.Debug("user exists result: ", "exists", exists)
	return exists, nil
}

func (s *SQLiteStore) UpdateUser(user *models.User) error {
	slog.Debug("updating user", "usr", fmt.Sprintf("%+v", user))
	query := `
    UPDATE users
    SET strava_id = ?, telegram_chat_id = ?, username = ?, email = ?,
      strava_refresh_token = ?, strava_access_token = ?, strava_access_code = ?, token_expires_at = ?, language = ?
    WHERE id = ?
  `
	_, err := s.DB.Exec(query, user.StravaId, user.TelegramChatId, user.Username, user.Email,
		user.StravaRefreshToken, user.StravaAccessToken, user.StravaAccessCode, user.TokenExpiresAt, user.Language, user.ID)
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
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

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
