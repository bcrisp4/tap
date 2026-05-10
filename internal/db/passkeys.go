package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Passkey struct {
	ID           int64
	UserID       int64
	CredentialID []byte
	PublicKey    []byte
	SignCounter  int64
	AAGUID       []byte
	Label        string
	CreatedAt    int64
}

func InsertPasskey(ctx context.Context, d *sql.DB, p Passkey) (int64, error) {
	res, err := d.ExecContext(ctx,
		`INSERT INTO passkeys (user_id, credential_id, public_key, sign_counter, aaguid, label, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.CredentialID, p.PublicKey, p.SignCounter, p.AAGUID, p.Label, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("insert passkey: %w", err)
	}
	return res.LastInsertId()
}

func GetPasskeysByUserID(ctx context.Context, d *sql.DB, userID int64) ([]Passkey, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, user_id, credential_id, public_key, sign_counter, aaguid, label, created_at
		 FROM passkeys WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("get passkeys by user: %w", err)
	}
	defer rows.Close()
	var out []Passkey
	for rows.Next() {
		var p Passkey
		if err := rows.Scan(&p.ID, &p.UserID, &p.CredentialID, &p.PublicKey,
			&p.SignCounter, &p.AAGUID, &p.Label, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan passkey: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func GetPasskeyByCredentialID(ctx context.Context, d *sql.DB, credentialID []byte) (Passkey, error) {
	var p Passkey
	err := d.QueryRowContext(ctx,
		`SELECT id, user_id, credential_id, public_key, sign_counter, aaguid, label, created_at
		 FROM passkeys WHERE credential_id = ?`, credentialID).
		Scan(&p.ID, &p.UserID, &p.CredentialID, &p.PublicKey,
			&p.SignCounter, &p.AAGUID, &p.Label, &p.CreatedAt)
	if err != nil {
		return Passkey{}, err
	}
	return p, nil
}

func UpdatePasskeySignCounter(ctx context.Context, d *sql.DB, id, counter int64) error {
	_, err := d.ExecContext(ctx,
		`UPDATE passkeys SET sign_counter = ? WHERE id = ?`, counter, id)
	if err != nil {
		return fmt.Errorf("update passkey sign counter: %w", err)
	}
	return nil
}

func DeletePasskey(ctx context.Context, d *sql.DB, id, userID int64) error {
	res, err := d.ExecContext(ctx,
		`DELETE FROM passkeys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete passkey: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
