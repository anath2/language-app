package translation

import (
	"fmt"
	"time"
)

func (s *ProfileStore) UpsertUserProfile(name string, email string, language string) (UserProfile, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE user_profile SET name = ?, email = ?, language = ?, updated_at = ? WHERE id = 1`,
		name, email, language, now)
	if err != nil {
		return UserProfile{}, fmt.Errorf("update user profile: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		if _, err := s.db.Exec(`INSERT INTO user_profile (id, name, email, language, created_at, updated_at) VALUES (1, ?, ?, ?, ?, ?)`,
			name, email, language, now, now); err != nil {
			return UserProfile{}, fmt.Errorf("insert user profile: %w", err)
		}
	}
	return UserProfile{Name: name, Email: email, Language: language, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *ProfileStore) GetUserProfile() (UserProfile, bool) {
	row := s.db.QueryRow(`SELECT name, email, language, created_at, updated_at FROM user_profile WHERE id = 1`)
	var p UserProfile
	if err := row.Scan(&p.Name, &p.Email, &p.Language, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return UserProfile{}, false
	}
	return p, true
}
