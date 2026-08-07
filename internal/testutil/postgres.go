// Package testutil contains helpers shared by integration tests.
package testutil

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// OpenTestDB opens TEST_DATABASE_URL and refuses to clean a database whose name
// does not clearly identify it as test-only. Tests skip when the variable is absent.
func OpenTestDB(t testing.TB) (*pgxpool.Pool, *db.Queries) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration test skipped: TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(dsn)
	require.NoError(t, err, "parse TEST_DATABASE_URL")
	databaseName := strings.TrimPrefix(parsed.Path, "/")
	require.Contains(t, strings.ToLower(databaseName), "test", "refusing destructive integration tests against non-test database %q", databaseName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)
	lockConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = lockConn.Exec(ctx, `SELECT pg_advisory_lock(710071)`)
	require.NoError(t, err)

	var jobsTableExists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'jobs'
	)`).Scan(&jobsTableExists)
	require.NoError(t, err)
	require.True(t, jobsTableExists, "jobs table is missing; apply internal/db/migrations before running integration tests")

	queries := db.New(pool)
	require.NoError(t, queries.RemoveJobsFromDBForTest(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		require.NoError(t, queries.RemoveJobsFromDBForTest(cleanupCtx))
		_, unlockErr := lockConn.Exec(cleanupCtx, `SELECT pg_advisory_unlock(710071)`)
		require.NoError(t, unlockErr)
		lockConn.Release()
	})
	return pool, queries
}
