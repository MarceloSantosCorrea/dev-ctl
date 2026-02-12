package database

import "database/sql"

// GetSetting returns the value of a setting, or defaultVal if not found.
func GetSetting(db *sql.DB, key, defaultVal string) string {
	var val string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

// SetSetting inserts or updates a setting.
func SetSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}
