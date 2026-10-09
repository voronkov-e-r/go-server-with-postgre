package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string

var ErrNotFound = errors.New("not found")

const (
	StatusAvailable Status = "available"
	StatusBorrowed  Status = "borrowed"
	StatusLost      Status = "lost"
)

func (st *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch Status(str) {
	case StatusAvailable, StatusBorrowed, StatusLost:
		*st = Status(str)
		return nil
	default:
		return fmt.Errorf("unknown status: %q", str)
	}
}

type Book struct {
	ID        int64      `json:"id"        db:"id"`
	Title     string     `json:"title"     db:"title"`
	Author    string     `json:"author"    db:"author"`
	Year      int        `json:"year"      db:"year"`
	Status    Status     `json:"status"    db:"status"`
	Tags      []string   `json:"tags"      db:"tags"`
	Publisher *Publisher `json:"publisher,omitempty" db:"publisher"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

type Publisher struct {
	Name    string `json:"name"`
	Country string `json:"country,omitempty"`
}

type Server struct {
	store *Store
}

func (s *Server) handleGetBooks(w http.ResponseWriter, r *http.Request) {
	reqId, _ := r.Context().Value(requestIDKey).(string)
	books, err := s.store.List(r.Context())

	if err != nil {
		slog.Error("db error", "err", err, "id", reqId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(books)
}

func (s *Server) handleGetBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	reqId, _ := r.Context().Value(requestIDKey).(string)

	book, err := s.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("db error", "err", err, "id", reqId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

func (s *Server) handleCreateBook(w http.ResponseWriter, r *http.Request) {
	var book Book
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&book); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if book.Title == "" || book.Author == "" {
		http.Error(w, "Incorrect input", http.StatusBadRequest)
		return
	}

	if book.Status == "" {
		book.Status = StatusAvailable
	}

	if book.Tags == nil {
		book.Tags = []string{}
	}

	reqId, _ := r.Context().Value(requestIDKey).(string)
	book, err := s.store.Create(r.Context(), book)

	if err != nil {
		slog.Error("db error", "err", err, "id", reqId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/books/%d", book.ID))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)
}

func (s *Server) handleUpdateBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	p, err := parseBookPatch(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	book, err := s.store.Update(r.Context(), int64(id), p)

	if errors.Is(err, ErrNotFound) {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	reqId, _ := r.Context().Value(requestIDKey).(string)
	if err != nil {
		slog.Error("db error", "err", err, "id", reqId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

func (s *Server) handleDeleteBook(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)

	if err != nil {
		http.Error(w, "Incorrect id", http.StatusBadRequest)
		return
	}

	ok := s.store.Delete(r.Context(), int64(id))

	reqId, _ := r.Context().Value(requestIDKey).(string)
	if errors.Is(ok, ErrNotFound) {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}
	if ok != nil {
		slog.Error("db error", "err", ok, "id", reqId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://bookapi:secret@localhost:5432/bookapi?sslmode=disable")
	if err != nil {
		slog.Error("failed to connect to db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("db ping failed", "err", err)
		os.Exit(1)
	}
	slog.Info("connected to postgres")

	store := NewStore(pool)
	srv := &Server{store: store}

	mux := http.NewServeMux()

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	mux.HandleFunc("GET /books", srv.handleGetBooks)
	mux.HandleFunc("GET /books/{id}", srv.handleGetBook)
	mux.HandleFunc("POST /books", srv.handleCreateBook)
	mux.HandleFunc("PUT /books/{id}", srv.handleUpdateBook)
	mux.HandleFunc("DELETE /books/{id}", srv.handleDeleteBook)
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			fmt.Fprintln(w, "done")
		case <-r.Context().Done():
			slog.Info("client disconnected", "path", "/slow")
		}
	})

	handle := Chain(mux, Logging, Recovery, RequestID)

	httpSrv := &http.Server{
		Addr:         ":8080",
		Handler:      handle,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", "8080")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "err", err)
	}
	slog.Info("server stopped")
}
