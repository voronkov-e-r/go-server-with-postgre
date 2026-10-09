package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Get(ctx context.Context, id int64) (Book, error) {
	var b Book
	err := s.db.QueryRow(ctx, `
        SELECT id, title, author, year, status, tags, publisher, created_at, updated_at
        FROM books WHERE id = $1
    `, id).Scan(&b.ID, &b.Title, &b.Author, &b.Year, &b.Status, &b.Tags, &b.Publisher, &b.CreatedAt, &b.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, err
}

func (s *Store) Create(ctx context.Context, b Book) (Book, error) {
	err := s.db.QueryRow(ctx, `
        INSERT INTO books (title, author, year, status, tags, publisher)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at, updated_at
    `, b.Title, b.Author, b.Year, b.Status, b.Tags, b.Publisher).
		Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (s *Store) Update(ctx context.Context, id int64, p *bookPatch) (Book, error) {
	sets := []string{}
	args := []any{}
	i := 1

	if p.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", i))
		args = append(args, *p.Title)
		i++
	}
	if p.Author != nil {
		sets = append(sets, fmt.Sprintf("author = $%d", i))
		args = append(args, *p.Author)
		i++
	}
	if p.Year != nil {
		sets = append(sets, fmt.Sprintf("year = $%d", i))
		args = append(args, *p.Year)
		i++
	}
	if p.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", i))
		args = append(args, *p.Status)
		i++
	}
	if p.Tags != nil {
		sets = append(sets, fmt.Sprintf("tags = $%d", i))
		args = append(args, p.Tags)
		i++
	}
	if p.Publisher != nil {
		sets = append(sets, fmt.Sprintf("publisher = $%d", i))
		args = append(args, p.Publisher)
		i++
	}

	if len(sets) == 0 {
		return s.Get(ctx, id)
	}

	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf(`
        UPDATE books SET %s
        WHERE id = $%d
        RETURNING id, title, author, year, status, tags, publisher, created_at, updated_at
    `, strings.Join(sets, ", "), i)

	var b Book
	err := s.db.QueryRow(ctx, query, args...).Scan(
		&b.ID, &b.Title, &b.Author, &b.Year, &b.Status, &b.Tags, &b.Publisher, &b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, err
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, "DELETE FROM books WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context) ([]Book, error) {
	rows, err := s.db.Query(ctx, `
        SELECT id, title, author, year, status, tags, publisher, created_at, updated_at
        FROM books ORDER BY id
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := []Book{}
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Year, &b.Status, &b.Tags, &b.Publisher, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
